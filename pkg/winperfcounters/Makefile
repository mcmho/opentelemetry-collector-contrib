include ../../Makefile.Common

# Remove "-race" from the default set of test arguments.
# pkg/winperfcounters tests are failing with the -race check.
# See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10145
GOTEST_OPT = -v -timeout 300s --tags=$(GO_BUILD_TAGS)
