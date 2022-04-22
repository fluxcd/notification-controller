module github.com/fluxcd/notification-controller/tests/fuzz

// This module is used only to avoid polluting the main module
// with fuzz dependencies.

go 1.17

replace (
	github.com/fluxcd/notification-controller/api => ../../api
	github.com/fluxcd/notification-controller => ../../
)
