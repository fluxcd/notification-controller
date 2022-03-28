#!/usr/bin/env bash

# Copyright 2022 The Flux authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euxo pipefail

GOPATH="${GOPATH:-/root/go}"
GO_SRC="${GOPATH}/src"
PROJECT_PATH="github.com/fluxcd/notification-controller"

cd "${GO_SRC}"

# Move fuzzer to their respective directories.
# This removes dependency noises from the modules' go.mod and go.sum files.
cp "${PROJECT_PATH}/tests/fuzz/util_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/alertmanager_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/opsgenie_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/webex_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/discord_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/forwarder_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/lark_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/matrix_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/rocket_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/slack_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/teams_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/google_chat_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/azure_devops_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/bitbucket_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/github_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"
cp "${PROJECT_PATH}/tests/fuzz/gitlab_fuzzer.go" "${PROJECT_PATH}/internal/notifier/"


# compile fuzz tests for the runtime module
pushd "${PROJECT_PATH}"

go get -d github.com/AdaLogics/go-fuzz-headers
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzNotifierUtil fuzz_notifier_util
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzAlertmanager fuzz_alert_manager
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzOpsGenie fuzz_opsgenie
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzWebex fuzz_webex
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzDiscord fuzz_discord
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzForwarder fuzz_forwarder
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzLark fuzz_lark
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzMatrix fuzz_matrix
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzRocket fuzz_rocket
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzSlack fuzz_slack
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzMSTeams fuzz_msteams
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzGoogleChat fuzz_google_chat
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzAzureDevOps fuzz_azure_devops
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzBitbucket fuzz_bitbucket
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzGitHub fuzz_github
compile_go_fuzzer "${PROJECT_PATH}/internal/notifier/" FuzzGitLab fuzz_gitlab

popd
