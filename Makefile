GO = go
LDFLAGS = -s -w

CNAMESFN = pkg/keyboard/colornames.csv.gz

APPNAME = $(shell awk '/^name:/{print $$2}' buildinfo/app.yml)
VERSION = $(strip $(shell git describe --tags --abbrev=0 --dirty --always))
COMMITHASH = $(strip $(shell git rev-parse --short HEAD))
BUILDTIME = $(strip $(shell date -u +%Y-%m-%dT%H:%M:%SZ))

build: $(CNAMESFN) $(APPNAME)

$(APPNAME): $(shell find -name '*.go')
	$(RM) $@ # to make it obvious when watch fails
	$(GO) generate ./...
	$(GO) build -ldflags='$(LDFLAGS)' -o $@

watch: .reflex_installed build
	reflex -r '\.(go)$$' -d fancy $(MAKE) build

tag-%:
	./scripts/bump.sh -p v $*
	echo -n "git push origin v$$(./scripts/bump.sh -p v -l)" | xclip -sel clip -i

appname:
	@echo -n $(APPNAME)

define BUILDINFO
build_time: $(BUILDTIME)
commit_hash: $(COMMITHASH)
version: $(VERSION)
endef

export BUILDINFO
buildinfo:
	echo "$$BUILDINFO" | tee buildinfo/build.yml

# grab the list of simple color names (the full list is quite large)
$(CNAMESFN):
	curl -sS https://raw.githubusercontent.com/meodai/color-names/master/src/colornames.csv | \
	  awk -F, '$$1 ~ /^[a-zA-Z]+$$/' | \
	  gzip > $@

.reflex_installed:
	which reflex >/dev/null 2>&1 || ( cd ~ && go install github.com/cespare/reflex@latest )
	touch $@

clean:
	$(RM) .reflex_installed $(APPNAME) buildinfo/build.yml $(CNAMESFN)

.PHONY: build appname buildinfo clean
