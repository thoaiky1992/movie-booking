package application

type HealthChecker interface {
	Health() map[string]string
}
