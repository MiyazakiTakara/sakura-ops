# Supply-chain targets (NEXT-005). Included from the root Makefile.
SYFT      := bin/tools/syft
SBOM_DIR  := bin/sbom
SBOMS     := $(SBOM_DIR)/go.cdx.json $(SBOM_DIR)/npm-prod.cdx.json $(SBOM_DIR)/npm-dev.cdx.json $(SBOM_DIR)/server-binary.cdx.json

.PHONY: sbom license-check deps-audit supply-chain image-scan

## sbom: CycloneDX SBOMs for Go, npm (prod and dev) and the built server binary, in bin/sbom/.
sbom: build
	./scripts/install-syft.sh
	./scripts/sbom.sh

## license-check: fail on denied, unknown or unlisted licences (supply-chain/license-policy.json).
license-check: sbom
	$(GO) run ./cmd/licensecheck -policy supply-chain/license-policy.json $(SBOMS)

## deps-audit: govulncheck plus npm audit (prod, then prod+dev); fails on high or critical.
deps-audit: vuln
	cd frontend && npm audit --omit=dev --audit-level=high
	cd frontend && npm audit --audit-level=high

## supply-chain: SBOM, licence policy and vulnerability audit.
supply-chain: license-check deps-audit

## image-scan: scan every image of compose.dev.yaml (built targets, pinned externals) with grype (needs docker, network).
## Fails on HIGH/CRITICAL with a fix, warns on unfixed ones; exceptions only via supply-chain/image-scan-ignore.json.
image-scan:
	./scripts/install-syft.sh
	./scripts/install-grype.sh
	GO='$(GO)' ./scripts/image-scan.sh
