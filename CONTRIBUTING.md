# Contributing to Whisper Remote

Thanks for your interest in contributing!

## How to Contribute

1. **Fork** the repository
2. **Create a branch** for your feature or fix
3. **Make your changes**
4. **Test** your changes locally
5. **Submit a Pull Request**

## Requirements

### Code Standards

- Keep it simple - this is a ~400 line app, don't over-engineer
- Follow existing code style (gofmt)
- No external dependencies unless absolutely necessary

### Pull Requests

- One feature/fix per PR
- Clear description of what and why
- Test on Windows before submitting

### Commit Messages

- Use present tense ("Add feature" not "Added feature")
- Keep first line under 72 characters
- Reference issues if applicable

## Building

```bash
go build -ldflags="-H=windowsgui" -o whisper-remote.exe .
```

## Questions?

Open an issue on GitHub.
