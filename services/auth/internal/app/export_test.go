package app

// Test-only access to internals for the external app_test package.

// AccountKey exposes accountKey.
var AccountKey = accountKey

// DummyHash returns the dummy hash used for unknown accounts.
func DummyHash(s *Service) string { return s.dummyHash }
