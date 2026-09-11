// Package config resolves the endpoint and auth settings per field, with
// precedence env var > active profile > compiled-in production default, and
// owns the profiles.json file the profiles are read from.
package config
