package utils

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxRetries int           // Maximum number of retries
	BaseDelay  time.Duration // Base delay between retries
	MaxDelay   time.Duration // Maximum delay between retries
	Multiplier float64       // Exponential backoff multiplier
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  time.Second,
		MaxDelay:   time.Minute,
		Multiplier: 2.0,
	}
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary() || netErr.Timeout()
	}

	// Check for specific error messages that indicate retryable conditions
	errStr := err.Error()
	retryableErrors := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"rate limit",
		"too many requests",
		"service unavailable",
		"internal server error",
		"bad gateway",
		"gateway timeout",
		"network is unreachable",
		"slack rate limit",
		"rate_limited",
		"ratelimited",
		"429",
		"too_many_requests",
	}

	// Check for permanent errors that should NOT be retried
	permanentErrors := []string{
		"is_archived",
		"not_in_channel",
		"channel_not_found",
		"cant_invite_self",
		"invalid_auth",
		"account_inactive",
		"token_revoked",
	}

	for _, permanentErr := range permanentErrors {
		if strings.Contains(strings.ToLower(errStr), permanentErr) {
			return false // Don't retry permanent errors
		}
	}

	for _, retryableErr := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryableErr) {
			return true
		}
	}

	return false
}

// GetRetryDelay calculates the appropriate delay for retrying based on error type
func GetRetryDelay(err error, attempt int, baseDelay time.Duration) time.Duration {
	if err == nil {
		return baseDelay
	}

	errStr := strings.ToLower(err.Error())

	// Slack rate limiting - use longer delays
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too_many_requests") || strings.Contains(errStr, "ratelimited") {
		// Slack typically requires longer waits for rate limits
		delay := time.Duration(attempt) * 5 * time.Second
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		return delay
	}

	// Network errors - moderate delays
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection") {
		delay := time.Duration(attempt) * 2 * time.Second
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		return delay
	}

	// Default exponential backoff
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
	if delay > 2*time.Minute {
		delay = 2 * time.Minute
	}
	return delay
}

// RetryWithBackoff executes a function with exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, config RetryConfig, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay based on error type and attempt number
			delay := GetRetryDelay(lastErr, attempt-1, config.BaseDelay)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}

			// Add jitter to prevent thundering herd
			jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
			delay += jitter

			logrus.Debugf("Retry attempt %d/%d after %v (last error: %v)",
				attempt+1, config.MaxRetries+1, delay, lastErr)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := operation()
		if err == nil {
			if attempt > 0 {
				logrus.Debugf("Operation succeeded on attempt %d", attempt+1)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err) {
			logrus.Debugf("Error is not retryable: %v", err)
			return err
		}

		if attempt == config.MaxRetries {
			logrus.Warnf("Max retries (%d) exceeded, giving up. Last error: %v", config.MaxRetries, err)
			break
		}

		logrus.Debugf("Attempt %d failed with retryable error: %v", attempt+1, err)
	}

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries+1, lastErr)
}
