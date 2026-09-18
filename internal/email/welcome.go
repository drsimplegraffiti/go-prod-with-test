package email

type WelcomeEmailData struct {
	Name  string
	Email string
}

func (s *Service) SendWelcomeEmail(to, name string) {
	s.SendAsync(
		to,
		"Welcome to Our Platform",
		"internal/email/templates/welcome.html",
		WelcomeEmailData{
			Name:  name,
			Email: to,
		},
	)
}
