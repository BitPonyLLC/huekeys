GO = go
LDFLAGS = -s -w

APPNAME := $(shell sed -n 's/^.*Name = "\([^"]*\)".*$$/\1/p' buildinfo/buildinfo.go)
CNAMESFN := pkg/keyboard/colornames.csv.gz

build: $(CNAMESFN) $(APPNAME)

$(APPNAME): $(shell find -name '*.go')
	$(RM) $@ # to make it obvious when watch fails
	$(GO) generate ./...
	$(GO) build -ldflags='$(LDFLAGS)' -o $@

watch: .reflex_installed build
	reflex -r '\.(go)$$' -d fancy -- sh -c '$(MAKE) build && cat buildinfo/build_time.txt'

# grab the list of simple color names (the full list is quite large)
$(CNAMESFN):
	curl -sS https://raw.githubusercontent.com/meodai/color-names/master/src/colornames.csv | \
	  awk -F, '$$1 ~ /^[a-zA-Z]+$$/' | \
	  gzip > $@

.reflex_installed:
	which reflex >/dev/null 2>&1 || ( cd ~ && go install github.com/cespare/reflex@latest )
	touch $@

clean:
	$(RM) .reflex_installed $(APPNAME) buildinfo/version.txt $(CNAMESFN)

.PHONY: clean