# 🚀 Contributing Manual

This project is open to contributions from all levels and any help is appreciated! 

> [!Tip]
>
> **For new contributors:** Take a look at [https://github.com/firstcontributions/first-contributions](https://github.com/firstcontributions/first-contributions) for a simple quick start guide on GitHub contributions.

## Development

### Prerequisites

Install the following tools for the project:

- [Go](https://go.dev/dl/) - API server and server-rendered dashboard (core)
- [gcc](https://gcc.gnu.org/install/) - Required for CGO in Go (core)
- [mise-en-place](https://mise.jdx.dev/) - Tool and workflow manager

MedamaLite does **not** require Node.js, npm or Bun. The tracker scripts are frozen
inside `core/client/scripts` and the dashboard is rendered server-side by the Go binary.

We recommend using [mise-en-place](https://mise.jdx.dev/) as a convenient version manager tool for all the languages and tools required for this project.

### Setup

We use mise tasks to run project workflows. Run `mise install` from the repository root first, then use the package tasks for normal development.

To start the API and the server-rendered dashboard (a single binary), run:

```bash
mise dev
```

To seed local databases with deterministic dashboard data before starting the app, run:

```bash
mise run seed -- --reset
```

More details on the development setup can be found in the core README.

- [Core](./core/README.md)
