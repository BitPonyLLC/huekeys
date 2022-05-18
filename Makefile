GO = go
LDFLAGS = -s -w

build: pkg/colornames.csv.gz
	$(GO) build -ldflags='$(LDFLAGS)'

watch: .reflex_installed build
	reflex -r '\.(go)$$' -d fancy -- $(MAKE) build

# grab the list of simple color names (the full list is quite large)
pkg/colornames.csv.gz:
	curl -sS https://raw.githubusercontent.com/meodai/color-names/master/src/colornames.csv | \
	  awk -F, '$$1 ~ /^[a-zA-Z]+$$/' | \
	  gzip > $@

.reflex_installed:
	which reflex >/dev/null 2>&1 || ( cd ~ && go install github.com/cespare/reflex@latest )
	touch $@