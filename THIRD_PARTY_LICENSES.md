# Third-Party License Notes

This project uses third-party Go modules. A license compatibility check was run with:

```bash
go run github.com/google/go-licenses@latest report ./...
go run github.com/google/go-licenses@latest check ./... --allowed_licenses=MIT,BSD-2-Clause,BSD-3-Clause,Apache-2.0,ISC
```

## Compatibility status

- Detected licenses are permissive and MIT-compatible (`MIT`, `BSD-3-Clause`).
- No copyleft licenses (GPL/LGPL/AGPL) were found in the scanned dependency set.
