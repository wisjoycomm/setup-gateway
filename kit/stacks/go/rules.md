## Go
- Run `gofmt -l .`, `go vet ./...` and `go test ./...` before calling a change done.
- Never edit `go.sum` by hand. Run `go mod tidy` only when dependencies changed.
- Wrap errors with context (`fmt.Errorf("...: %w", err)`) and handle them; don't discard them without a reason.
- Prefer the standard library. Don't add a dependency without saying why in your report.
- Use `t.TempDir()` and `t.Setenv` in tests, so they never touch the real machine.
