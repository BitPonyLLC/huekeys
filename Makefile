GO = go
LDFLAGS = -s -w

CNAMESFN := pkg/keyboard/colornames.csv.gz

build: $(CNAMESFN)
	$(GO) generate ./...
	$(GO) build -ldflags='$(LDFLAGS)'

watch: .reflex_installed build
	reflex -r '\.(go)$$' -d fancy -- $(MAKE) build

# grab the list of simple color names (the full list is quite large)
$(CNAMESFN):
	curl -sS https://raw.githubusercontent.com/meodai/color-names/master/src/colornames.csv | \
	  awk -F, '$$1 ~ /^[a-zA-Z]+$$/' | \
	  gzip > $@

.reflex_installed:
	which reflex >/dev/null 2>&1 || ( cd ~ && go install github.com/cespare/reflex@latest )
	touch $@

clean:
	$(RM) .reflex_installed huekeys buildinfo/version.txt $(CNAMESFN)
