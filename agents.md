# Agent Instructions - SOLID & Hexagonal Guidelines

## Mandatory Architecture

1.  **Hexagonal Isolation**: Never import `internal/adapters/...` from `internal/domain/...`.
2.  **SOLID Principles**:
    *   **SRP**: Each file should have one clear purpose. Handlers handle HTTP, Clients handle API calls, Strategies handle translation.
    *   **OCP**: Use the Factory pattern for adding new device types.
    *   **DIP**: Always inject dependencies via Port interfaces.
3.  **Huego library**: Use `github.com/amimof/huego` for Hue-related models.
4.  **Asynchrony**: HA service calls MUST be asynchronous to keep the Hue API responsive for Alexa.

## Domain Logic

*   **Virtual Devices**: The system supports multiple "virtual intentions" for a single Home Assistant entity.
*   **Translation Engine**: The `CustomStrategy` evaluates formulas where `x` is the input value.
*   **Bridge Service**: Handles optimistic state updates and stable numeric HueID generation.

## Compliance

*   Run `make test` before every commit.
*   ArchUnit tests MUST pass.
*   Maintain > 80% coverage in `internal/domain/service` and `internal/domain/translator`.
