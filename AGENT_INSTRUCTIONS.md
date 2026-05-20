# Agent Instructions

Use `aget` for browser work. Managed browser installs use CloakBrowser stealth Chromium by default.

Start with `aget open URL -n NAME`. Use the returned `sid` for `aget page read`, `aget page click`, `aget page type`, `aget page screenshot`, and `aget session close`.

Do not paste secrets into examples or logs. Put secrets into environment variables and pass them to commands only when needed.
