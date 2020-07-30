# notification-controller

[![e2e](https://github.com/fluxcd/notification-controller/workflows/e2e/badge.svg)](https://github.com/fluxcd/notification-controller/actions)
[![report](https://goreportcard.com/badge/github.com/fluxcd/notification-controller)](https://goreportcard.com/report/github.com/fluxcd/notification-controller)
[![license](https://img.shields.io/github/license/fluxcd/notification-controller.svg)](https://github.com/fluxcd/notification-controller/blob/master/LICENSE)
[![release](https://img.shields.io/github/release/fluxcd/notification-controller/all.svg)](https://github.com/fluxcd/notification-controller/releases)

Experimental event forwarded and notification dispatcher for the GitOps Toolkit controllers.
The notification-controller is an implementation of the [notification.toolkit.fluxcd.io](docs/spec/v1alpha1/README.md)
API based on the specifications described in the [RFC](docs/spec/README.md).

![overview](docs/diagrams/notification-controller-overview.png)
