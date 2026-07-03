package client

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcclient"
)

// AuthSession содержит токены, пользовательскую соль и верификатор мастер-ключа, полученные после аутентификации.
type AuthSession struct {
	AccessToken       string
	RefreshToken      string
	MasterKeySalt     []byte
	MasterKeyVerifier []byte
}

// Register регистрирует пользователя на сервере.
func (a *App) Register(ctx context.Context, login, password, masterKey string) (AuthSession, error) {
	salt, err := crypto.GenerateMasterKeySalt()
	if err != nil {
		return AuthSession{}, fmt.Errorf("не удалось сгенерировать соль мастер-ключа: %w", err)
	}
	verifier, err := crypto.EncryptMasterKeyVerifier(masterKey, salt)
	if err != nil {
		return AuthSession{}, fmt.Errorf("не удалось зашифровать проверочные данные мастер-ключа: %w", err)
	}

	resp, err := a.auth.Register(ctx, pb.RegisterRequest_builder{
		Login:             &login,
		Password:          &password,
		MasterKeySalt:     salt,
		MasterKeyVerifier: verifier.Data,
	}.Build())
	if err != nil {
		return AuthSession{}, rpcError(err, "не удалось зарегистрировать пользователя", map[codes.Code]string{
			codes.AlreadyExists:   "логин уже занят",
			codes.InvalidArgument: "некорректные регистрационные данные",
		})
	}
	return AuthSession{
		AccessToken:       resp.GetAccessToken(),
		RefreshToken:      resp.GetRefreshToken(),
		MasterKeySalt:     salt,
		MasterKeyVerifier: verifier.Data,
	}, nil
}

// Login аутентифицирует пользователя на сервере.
func (a *App) Login(ctx context.Context, login, password string) (AuthSession, error) {
	resp, err := a.auth.Login(ctx, pb.LoginRequest_builder{
		Login:    &login,
		Password: &password,
	}.Build())
	if err != nil {
		return AuthSession{}, rpcError(err, "не удалось войти в аккаунт", map[codes.Code]string{
			codes.Unauthenticated: "неверный логин или пароль",
			codes.InvalidArgument: "некорректные данные для входа в аккаунт",
		})
	}
	return AuthSession{
		AccessToken:       resp.GetAccessToken(),
		RefreshToken:      resp.GetRefreshToken(),
		MasterKeySalt:     resp.GetMasterKeySalt(),
		MasterKeyVerifier: resp.GetMasterKeyVerifier(),
	}, nil
}

// StartSession проверяет мастер-ключ и сохраняет серверную сессию в памяти процесса.
func (a *App) StartSession(session AuthSession, masterKey string) error {
	if err := validateSessionTokens(session); err != nil {
		return err
	}
	if len(session.MasterKeySalt) == 0 {
		return errors.New("соль мастер-ключа отсутствует")
	}
	if len(session.MasterKeyVerifier) == 0 {
		return errors.New("проверочные данные мастер-ключа отсутствуют")
	}
	if masterKey == "" {
		return errors.New("мастер-ключ обязателен")
	}
	if err := crypto.VerifyMasterKey(
		masterKey,
		session.MasterKeySalt,
		model.EncryptedBlob{Data: session.MasterKeyVerifier},
	); err != nil {
		if errors.Is(err, crypto.ErrInvalidMasterKey) {
			return errors.New("неверный мастер-ключ")
		}
		return fmt.Errorf("не удалось проверить мастер-ключ: %w", err)
	}

	a.session = session
	a.masterKey = masterKey
	a.loggedIn = true
	return nil
}

func validateSessionTokens(session AuthSession) error {
	if session.AccessToken == "" {
		return errors.New("access-токен отсутствует")
	}
	if session.RefreshToken == "" {
		return errors.New("refresh-токен отсутствует")
	}
	return nil
}

// withAccessTokenRefresh выполняет запрос с текущим access-токеном.
// Если сервер вернул Unauthenticated, метод один раз обновляет пару токенов и повторяет исходный запрос.
func (a *App) withAccessTokenRefresh(ctx context.Context, call func(context.Context) error) error {
	if err := a.requireSession(); err != nil {
		return err
	}

	err := call(grpcclient.WithAccessToken(ctx, a.session.AccessToken))
	if status.Code(err) != codes.Unauthenticated {
		return err
	}

	if err = a.refreshSession(ctx); err != nil {
		return err
	}
	return call(grpcclient.WithAccessToken(ctx, a.session.AccessToken))
}

// withAccessTokenRefreshRetry выполняет читающий унарный gRPC-запрос с retry/backoff для временных сетевых ошибок.
func (a *App) withAccessTokenRefreshRetry(ctx context.Context, call func(context.Context) error) error {
	if err := a.requireSession(); err != nil {
		return err
	}

	err := retryUnary(ctx, func(ctx context.Context) error {
		return call(grpcclient.WithAccessToken(ctx, a.session.AccessToken))
	})
	if status.Code(err) != codes.Unauthenticated {
		return err
	}

	if err = a.refreshSession(ctx); err != nil {
		return err
	}
	return retryUnary(ctx, func(ctx context.Context) error {
		return call(grpcclient.WithAccessToken(ctx, a.session.AccessToken))
	})
}

func (a *App) refreshSession(ctx context.Context) error {
	refreshToken := a.session.RefreshToken
	resp, err := a.auth.Refresh(ctx, pb.RefreshRequest_builder{RefreshToken: &refreshToken}.Build())
	if err != nil {
		if status.Code(err) == codes.Unauthenticated {
			a.clearSession()
			return errors.New("сессия истекла, войдите в аккаунт снова")
		}
		return err
	}
	session := a.session
	session.AccessToken = resp.GetAccessToken()
	session.RefreshToken = resp.GetRefreshToken()
	if err = validateSessionTokens(session); err != nil {
		return err
	}

	a.session = session
	return nil
}

func (a *App) clearSession() {
	a.session = AuthSession{}
	a.masterKey = ""
	a.loggedIn = false
}

// Logout завершает текущую пользовательскую сессию и очищает ее из памяти процесса.
func (a *App) Logout(ctx context.Context) error {
	if !a.loggedIn {
		return nil
	}

	refreshToken := a.session.RefreshToken
	_, err := a.auth.Logout(ctx, pb.LogoutRequest_builder{RefreshToken: &refreshToken}.Build())
	if err != nil {
		return rpcError(err, "не удалось выйти из аккаунта", map[codes.Code]string{
			codes.Unauthenticated: "сессия уже завершена или недействительна",
			codes.InvalidArgument: "некорректные данные для выхода из аккаунта",
		})
	}

	a.clearSession()
	return nil
}
