package client

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// defaultUnaryRetryBackoff возвращает задержки между повторными попытками читающих унарных gRPC-запросов.
func defaultUnaryRetryBackoff() []time.Duration {
	return []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
	}
}

// retryUnary выполняет читающий унарный gRPC-запрос с дефолтной политикой повторов.
func retryUnary(ctx context.Context, call func(context.Context) error) error {
	return retryUnaryWithBackoff(ctx, defaultUnaryRetryBackoff(), call)
}

// retryUnaryWithBackoff повторяет запрос после временных ошибок, выдерживая переданные задержки между попытками.
func retryUnaryWithBackoff(
	ctx context.Context,
	backoff []time.Duration,
	call func(context.Context) error,
) error {
	for attempt := 0; ; attempt++ {
		err := call(ctx)
		if err == nil {
			return nil
		}
		if attempt >= len(backoff) || !isRetriableUnaryError(err) {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err = sleepContext(ctx, backoff[attempt]); err != nil {
			return err
		}
	}
}

// isRetriableUnaryError проверяет, относится ли ошибка унарного gRPC-запроса к временным сбоям.
func isRetriableUnaryError(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.ResourceExhausted, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

// sleepContext ожидает заданную задержку и завершает ожидание раньше, если context отменен.
func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
