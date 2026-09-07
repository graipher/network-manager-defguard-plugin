# Defguard NetworkManager VPN plugin

## Current implementation status

The first runnable slice now lives in this repository:

- `nm-defguard-import` imports or refreshes user-owned profiles through `nmcli`.
- `nm-defguard-service` runs Defguard authentication in the profile owner's desktop session and implements the NetworkManager VPN D-Bus lifecycle, minimal external-interface configuration, interface monitoring, and location-scoped disconnect.
- Focused tests cover Defguard JSON parsing, interface identification, dry-run imports, profile authorization, and D-Bus introspection XML.

Real Defguard and NetworkManager acceptance testing remains outstanding because `defguard-client` and libnm are not installed in the development environment.

## Objective

Expose enrolled Defguard VPN locations as persistent NetworkManager VPN profiles, alongside other VPN connections. Activating a profile should launch the existing Defguard authentication flow, including company SSO/OIDC, then establish the tunnel. Deactivation should disconnect that location.

NetworkManager supplies connection controls and status. Defguard continues to own authentication, device-posture checks, WireGuard interfaces, routes, DNS, and keys.

## Scope and starting assumptions

- Target Linux with NetworkManager and Defguard Desktop Client 2.1.0 or later. Verify the exact installed client before implementation.
- Reuse enrollment and configuration from the Defguard desktop app. Enrollment remains in that app.
- Start with external OIDC MFA and locations that require no interactive code. Preserve any server-required posture checks.
- Use the desktop `defguard-client` CLI, not the separate `dg` network-device client.
- Create user-restricted NetworkManager profiles that reference existing Defguard locations. Do not copy WireGuard keys or session preshared keys into profiles.
- Initial delivery: explicit profile import/refresh, VPN activation/deactivation, browser SSO, and status monitoring on one target desktop/distribution.
- Defer a connection editor, native TOTP/email/mobile-approval dialogs, automatic profile refresh, and broad distribution packaging until the basic integration works.

## Evidence and constraints

The NetBird NetworkManager plugin is a working architectural reference: it implements the NetworkManager VPN D-Bus lifecycle while delegating tunnel management to NetBird. It signals successful activation without asking NetworkManager to configure IP addresses, routes, or DNS.

Defguard 2.1.0, released September 4, 2026, includes these documented operations:

| Requirement | Existing command |
| --- | --- |
| List enrolled locations | `defguard-client list --json` |
| Connect by local location ID | `defguard-client connect --id 1 --json` |
| Read active connections | `defguard-client status --json` |
| Disconnect by local location ID | `defguard-client disconnect --id 1 --json` |

For external-MFA locations, the CLI starts authentication, opens the browser, and polls for completion using shared Defguard core code. Static WireGuard configuration import cannot replace that flow: Defguard MFA obtains a session preshared key after authorization.

Known issues to resolve before committing to a CLI-only implementation:

- The CLI must run as the enrolled desktop user, with access to that user's configuration and browser session. The NetworkManager service normally runs with system privileges.
- In 2.1.0, JSON status omits location IDs, although internal active-state structures contain them. Names alone are insufficient when locations share names.
- CLI startup performs configuration polling and stale-connection cleanup even for status commands. Frequent polling is not a passive operation.
- Cancellation can race with successful tunnel creation. Cleanup must target only the activation being cancelled.
- NetworkManager must leave Defguard-owned interfaces unmanaged without excluding unrelated WireGuard connections.
- NetBird's single-active-engine restriction is specific to NetBird. Establish Defguard's actual concurrency behavior rather than copying that restriction blindly.

## Proposed architecture

```text
Desktop VPN menu / nmcli
          |
     NetworkManager
          | VPN D-Bus lifecycle
     Defguard VPN service
          | authenticated, user-scoped local control
     Desktop-session helper
          | defguard-client CLI
          +-- browser -> company SSO
          +-- Defguard service -> WireGuard, routes, DNS
```

Use a small session helper if necessary to cross the system/user boundary safely. Before adding a new transport, inspect NetworkManager's existing authentication-agent mechanisms and the NetBird implementation for reusable patterns. Select the smallest mechanism that supports connect, cancel, disconnect, and monitoring in the correct user session.

The service should reserve an activation, report starting, invoke the user's helper asynchronously, verify the resulting tunnel, and only then signal activation complete. It must propagate failures and release resources on every terminal path.

Profiles should carry a Defguard-specific service type, owning user, local location ID, and sufficient instance identity to detect stale or mismatched references. Names are display labels. Revalidate identity before connecting, especially after deletion or re-enrollment.

## Phase 1: Validate the backend and desktop integration

- [ ] Record the target distribution, desktop, NetworkManager version, and Defguard client/server versions.
- [ ] Verify that the installed client exposes the documented CLI commands and JSON formats.
- [ ] Inspect the relevant NetBird service, authentication helper, plugin metadata, and packaging source. Record the source revision used and retain required license notices if adapting code.
- [ ] Inspect the matching Defguard release's CLI, shared authentication code, and connection lifecycle.
- [ ] With a designated test location, verify CLI list/connect/status/disconnect from the enrolled user's session, including browser SSO and posture enforcement where configured.
- [ ] Check how CLI-created connections coexist with the GUI, whether they survive CLI exit, and how configuration refresh and session expiry behave.
- [ ] Resolve unambiguous status identity. Prefer an existing supported field or API; if unavailable, propose a minimal upstream JSON extension exposing target and instance identity. Do not guess by display name.
- [ ] Select a monitoring cadence or event source after measuring CLI polling cost and cleanup behavior.
- [ ] Verify that the target NetworkManager/desktop accepts the NetBird-style minimal activation configuration and displays the resulting profile among VPN connections.

Exit criterion: one real Defguard location can complete the required lifecycle through the CLI, and the user-session bridge, status identity, and NetworkManager activation contract are understood. Document any required upstream change before implementation depends on it.

## Phase 2: Import configured locations

- [ ] Implement an explicit import/refresh command that reads Defguard's structured location list as the enrolled user.
- [ ] Use NetworkManager's settings API or libnm to create user-restricted VPN profiles with stable mappings to Defguard locations.
- [ ] Label profiles with both instance and location names.
- [ ] Make repeated imports idempotent; update only profiles owned by this integration and preserve unrelated connections.
- [ ] Detect missing or changed locations. Report obsolete profiles rather than silently deleting them or remapping their IDs.
- [ ] Install the VPN service metadata needed for NetworkManager to recognize the profile type.

Exit criterion: imported locations remain visible while disconnected, duplicate location names remain distinguishable, and refreshing creates no duplicates.

## Phase 3: Connect and disconnect

- [ ] Implement the required NetworkManager VPN D-Bus methods, states, configuration signals, and failure signals, using NetBird as a reference.
- [ ] Implement the selected user-session bridge with authenticated callers and explicit ownership checks. Do not accept arbitrary executable paths, shell commands, or environment variables from profiles.
- [ ] Invoke Defguard with argument arrays and a validated location ID; keep browser authentication in the owning user's desktop session.
- [ ] Support asynchronous activation, bounded authentication waits, user cancellation, and normal disconnection.
- [ ] Confirm that successful CLI execution corresponds to the intended active interface before reporting success. Define readiness using Defguard state and, where appropriate, handshake evidence.
- [ ] Handle already-connected locations explicitly. Track whether the plugin created or adopted a connection so failed activation cannot tear down a pre-existing session accidentally.
- [ ] Disconnect only the selected location. Do not use `disconnect --all`.
- [ ] Report unsupported interactive MFA methods clearly until their dialogs are implemented.
- [ ] Leave interface, routing, DNS, and session-key management with Defguard. Exclude only validated Defguard interfaces from NetworkManager management, preserving existing unmanaged-device settings.

Exit criterion: selecting a VPN profile launches SSO when needed, reports connected after successful activation, and disconnects only that location when toggled off.

## Phase 4: Status and failure handling

- [ ] Monitor using the identity and mechanism validated in Phase 1.
- [ ] Reflect interface removal, authentication failure, session expiry, daemon unavailability, and user-triggered disconnection in NetworkManager state.
- [ ] Reconcile GUI disconnects of NetworkManager-started connections. For the first version, document whether GUI-started connections are adopted or simply detected when a profile is activated.
- [ ] Cover service/helper restart, desktop logout, and cancellation during tunnel creation. Avoid leaving a tunnel behind after a cancelled activation.
- [ ] Serialize conflicting operations on the same location. Add broader concurrency only when the tested backend supports it safely.
- [ ] Keep logs useful without exposing MFA tokens, browser authorization URLs containing tokens, private keys, or session preshared keys.

Exit criterion: NetworkManager status follows actual connection state, and failure or cancellation does not affect unrelated connections.

## Phase 5: Package and validate

- [ ] Package the service, session helper if needed, profile import command, NetworkManager metadata, and narrowly scoped D-Bus/service policies for the first target distribution.
- [ ] Document prerequisites, enrollment, import/refresh, normal operation, logs, known limits, and uninstall behavior.
- [ ] Add focused automated checks for profile identity/idempotence, backend result parsing, authorization boundaries, and activation/cancellation cleanup. Use a fake CLI backend for deterministic lifecycle tests.
- [ ] Run the relevant formatter, linter, compiler/type checks, and focused tests for the implementation language. Do not add Python tooling unless the implementation actually uses Python.
- [ ] Perform the real-desktop acceptance checks below with a designated test account/location. Coordinate disruptive daemon, network, or session tests before running them on a working VPN.

## Acceptance checks

| Scenario | Required result |
| --- | --- |
| Import while disconnected | Location appears as a persistent VPN profile in the target desktop and `nmcli` |
| Repeat import | No duplicate profiles; unrelated profiles unchanged |
| External SSO | Browser opens in the enrolled user's session; connection completes after authorization |
| Denied, expired, or cancelled authentication | Activation ends with an appropriate result; no unintended tunnel remains |
| Disconnect | Only the selected Defguard location disconnects |
| Disconnect through Defguard GUI | NetworkManager returns to disconnected |
| Duplicate location names | Correct instance/location is selected and monitored without guessing |
| Location removed or re-enrolled | Stale reference fails clearly instead of connecting elsewhere |
| Session expires or tunnel disappears | NetworkManager leaves the connected state |
| Helper or daemon unavailable | Action fails clearly and can be retried after recovery |
| Another VPN is active | Its profile and interface remain untouched; route/DNS interactions are documented |
| Required posture check fails | Connection remains denied by Defguard |
| Service restart or cancellation race | State reconciles without leaking or disconnecting unrelated tunnels |
| Different local user | Cannot access another user's Defguard configuration or control their location |

## Deferred work

- Native NetworkManager authentication dialogs for TOTP/email and mobile approval.
- GTK/KDE connection editors; an imported profile should not require an editor to activate.
- Automatic location synchronization and adoption of connections started outside NetworkManager.
- Additional distribution packages and desktop compatibility testing.
- Direct integration with Defguard's shared core or a dedicated control API, only if the CLI limitations justify it.

## Sources

- [NetBird NetworkManager VPN plugin](https://github.com/netbirdio/network-manager-vpn-plugin)
- [NetBird plugin architecture](https://github.com/netbirdio/network-manager-vpn-plugin/blob/main/docs/architecture.md)
- [NetBird plugin license](https://github.com/netbirdio/network-manager-vpn-plugin/blob/main/LICENSE)
- [Defguard Desktop Client 2.1.0 release](https://github.com/DefGuard/client/releases/tag/v2.1.0)
- [Defguard desktop CLI documentation](https://docs.defguard.net/using-defguard-for-end-users/desktop-client/command-line-defguard-cli)
- [Defguard external SSO MFA](https://docs.defguard.net/features/wireguard/multi-factor-authentication-mfa-2fa/external-sso-based-mfa)
- [Defguard MFA architecture](https://docs.defguard.net/in-depth/architecture/architecture)
- [Defguard NetworkManager interface conflict](https://docs.defguard.net/support-1/troubleshooting-guides/desktop-client/disconnecting-shows-connection-failed-linux-networkmanager)
- [Defguard 2.1.0 CLI authentication](https://github.com/DefGuard/client/blob/v2.1.0/src-tauri/client-cli/src/mfa.rs)
- [Defguard 2.1.0 CLI connection implementation](https://github.com/DefGuard/client/blob/v2.1.0/src-tauri/client-cli/src/commands/connect.rs)
- [Defguard 2.1.0 CLI status implementation](https://github.com/DefGuard/client/blob/v2.1.0/src-tauri/client-cli/src/commands/status.rs)
- [Defguard 2.1.0 CLI dispatch and startup cleanup](https://github.com/DefGuard/client/blob/v2.1.0/src-tauri/client-cli/src/lib.rs)

This plan is based on documentation and source inspection on September 7, 2026. No live company VPN integration has been tested yet.
