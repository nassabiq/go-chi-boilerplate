//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/nassabiq/go-chi-boilerplate/internal/dependency"
)

func InitializeApp() (*dependency.App, error) {
	wire.Build(dependency.ProviderSet)

	return nil, nil
}
