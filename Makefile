GO = go
LDFLAGS = -s -w

all: pkg/colornames.csv.gz
	$(GO) build -ldflags='$(LDFLAGS)'

# grab the list of simple color names (the full list is quite large)
pkg/colornames.csv.gz:
	curl -sS https://raw.githubusercontent.com/meodai/color-names/master/src/colornames.csv | \
	  awk -F, '$$1 ~ /^[a-zA-Z]+$$/' | \
	  gzip > $@
