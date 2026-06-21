.PHONY: build frontend backend clean

build: frontend backend

frontend:
	cd web && npm install && npm run build

backend:
	CGO_ENABLED=1 go build -o nimbusfs ./cmd/nimbusfs

clean:
	rm -f nimbusfs
	rm -rf web/dist
