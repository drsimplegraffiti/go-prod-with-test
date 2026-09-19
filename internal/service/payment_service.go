package service

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/example/goapi/internal/httpclient"
)

type PaymentService struct {
	client *httpclient.Client
}

type CreatePaymentRequest struct {
	Amount int
}

func NewPaymentService(
	client *httpclient.Client,
) *PaymentService {
	return &PaymentService{
		client: client,
	}
}

func (s *PaymentService) CreatePayment(
	ctx context.Context,
	req CreatePaymentRequest,
) error {
	response, err := s.client.Do(ctx, httpclient.Request{
		Method: http.MethodPost,
		URL:    "https://api.example.com/payments",

		Headers: http.Header{
			"Authorization": []string{
				"Bearer " + os.Getenv("PAYMENT_API_KEY"),
			},
		},

		Body: req,

		// Only enable this if the external API supports
		// idempotency for this operation.
		Retry: true,
	})
	if err != nil {
		return fmt.Errorf("create payment request: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf(
			"payment API returned status %d",
			response.StatusCode,
		)
	}

	return nil
}
