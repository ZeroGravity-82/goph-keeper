package service

import "zerogravity-82/goph-keeper/internal/pb"

func registerRequest(login, password string) *pb.RegisterRequest {
	return registerRequestWithMasterKeyData(
		login,
		password,
		[]byte("1234567890abcdef"),
		[]byte("verifier"),
	)
}

func registerRequestWithMasterKeyData(
	login string,
	password string,
	masterKeySalt []byte,
	masterKeyVerifier []byte,
) *pb.RegisterRequest {
	return pb.RegisterRequest_builder{
		Login:             &login,
		Password:          &password,
		MasterKeySalt:     masterKeySalt,
		MasterKeyVerifier: masterKeyVerifier,
	}.Build()
}
