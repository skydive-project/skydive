TEST_PATTERN?=
UT_PACKAGES?=$(shell $(GO) list ./... | grep -Ev '/tests|/contrib')
FUNC_TESTS_CMD:="grep -e 'func Test${TEST_PATTERN}' tests/*.go | perl -pe 's|.*func (.*?)\(.*|\1|g' | shuf"
FUNC_TESTS:=$(shell sh -c $(FUNC_TESTS_CMD))
VERBOSE_TESTS_FLAGS?=
TIMEOUT?=1m

ifeq ($(VERBOSE), true)
  VERBOSE_TESTS_FLAGS+=-test.v
endif

ifeq ($(WITH_OPENCONTRAIL), true)
  AGENT_TEST_EXTRA_PROBES+=opencontrail
endif

ifeq ($(WITH_VPP), true)
  AGENT_TEST_EXTRA_PROBES+=vpp
endif

ifeq ($(COVERAGE), true)
  TEST_COVERPROFILE?=../functionals.cover
  EXTRA_ARGS+=-test.coverprofile=${TEST_COVERPROFILE}
endif

ifeq ($(WITH_PROF), true)
  EXTRA_ARGS+=-profile
endif

ifeq ($(WITH_K8S), true)
  ANALYZER_TEST_PROBES+=k8s
endif

ifeq ($(WITH_ISTIO), true)
  ANALYZER_TEST_PROBES+=istio
endif

ifeq ($(WITH_OVN), true)
  ANALYZER_TEST_PROBES+=ovn
endif

ifeq ($(WITH_OVNK8S), true)
  ANALYZER_TEST_PROBES+=ovn
  ANALYZER_TEST_PROBES+=k8s
  ANALYZER_TEST_PROBES+=ovnk8s
  EXTRA_ARGS+=-analyzer.topology.ovn.address=$(OVN_ADDRESS)
endif


COVERAGE?=0
COVERAGE_MODE?=atomic
COVERAGE_WD?="."

comma:= ,
empty:=
space:= $(empty) $(empty)
EXTRA_ARGS+=-analyzer.topology.probes=$(subst $(space),$(comma),$(ANALYZER_TEST_PROBES)) -agent.topology.probes=$(subst $(space),$(comma),$(AGENT_TEST_EXTRA_PROBES))

.PHONY: test.functionals.clean
test.functionals.clean:
	rm -f tests/functionals

.PHONY: test.functionals.compile
test.functionals.compile: genlocalfiles
	$(GO) test -tags "${BUILD_TAGS} test" -race ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -c -o tests/functionals ./tests/

.PHONY: test.functionals.static
test.functionals.static: genlocalfiles
	$(GO) test -tags "netgo ${STATIC_BUILD_TAGS} test" \
		-ldflags "${LDFLAGS} -X $(SKYDIVE_GITHUB_VERSION) -extldflags \"-static $(STATIC_LIBS_ABS)\"" \
		-installsuffix netgo \
		-race ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} \
		-c -o tests/functionals ./tests/

ifeq (${DEBUG}, true)
define functionals_run
cd tests && sudo -E $$(which dlv) $(DLV_FLAGS) exec ./functionals -- $1
endef
else
define functionals_run
cd tests && sudo -E ./functionals $1
endef
endif

.PHONY: test.functionals.run
test.functionals.run:
	cd tests && sudo -E ./functionals ${VERBOSE_TESTS_FLAGS} -test.run "${TEST_PATTERN}" -test.timeout ${TIMEOUT} ${ARGS} ${EXTRA_ARGS}

.PHONY: tests.functionals.all
test.functionals.all: test.functionals.compile
	$(MAKE) TIMEOUT="8m" ARGS="${ARGS}" test.functionals.run EXTRA_ARGS="${EXTRA_ARGS}"

.PHONY: test.functionals.batch
test.functionals.batch: test.functionals.compile
	set -e ; $(MAKE) ARGS="${ARGS} " test.functionals.run EXTRA_ARGS="${EXTRA_ARGS}" TEST_PATTERN="${TEST_PATTERN}"

.PHONY: test.functionals
test.functionals: test.functionals.compile
	for functest in ${FUNC_TESTS} ; do \
		$(MAKE) ARGS="-test.run $$functest$$\$$ ${ARGS}" test.functionals.run EXTRA_ARGS="${EXTRA_ARGS}"; \
	done

.PHONY: functional
functional:
	$(MAKE) test.functionals VERBOSE=true TIMEOUT=10m ARGS='-standalone -analyzer.topology.backend elasticsearch -analyzer.flow.backend elasticsearch' TEST_PATTERN="${TEST_PATTERN}"

.PHONY: test
test: genlocalfiles
ifeq ($(COVERAGE), true)
	set -v ; \
	for pkg in ${UT_PACKAGES}; do \
		if [ -n "$$pkg" ]; then \
			coverfile="${COVERAGE_WD}/$$(echo $$pkg | tr / -).cover"; \
			$(GO) test -tags "${BUILD_TAGS} test" -covermode=${COVERAGE_MODE} -coverprofile="$$coverfile" ${VERBOSE_FLAGS} -timeout ${TIMEOUT} $$pkg; \
		fi; \
	done
else
ifneq ($(TEST_PATTERN),)
	set -v ; \
	$(GO) test -tags "${BUILD_TAGS} test" -ldflags="${LDFLAGS}" -race ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -test.run ${TEST_PATTERN} ${UT_PACKAGES}
else
	set -v ; \
	$(GO) test -tags "${BUILD_TAGS} test" -ldflags="${LDFLAGS}" -race ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} ${UT_PACKAGES}
endif
endif
