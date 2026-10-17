# ABAP dialog events with a Nanz TUI

This is an inspected integration design, not an implemented dynpro runtime.
It follows the native ABAP semantic hardening goal; it does not replace its
numeric, character, expression or Z80 acceptance requirements.

## What exists

MinZ lowers report execution as INITIALIZATION, selection-screen input,
AT-SELECTION-SCREEN, START-OF-SELECTION and END-OF-SELECTION. This is a
one-shot report path, not a general PBO/PAI dialog loop. Native selection
input currently uses fallback prompts; the sel_* registration functions are
not a native connection to the Nanz Screen implementation.

Nanz already supplies Screen.render/show, field descriptors, focus, keyboard
input, dialogs and an app_run screen stack in stdlib/tui. app_run renders
and dispatches its built-in navigation keys, but does not yet dispatch ABAP
PBO/PAI handlers. Screen.get_int currently returns the stored def_int;
editable integer transfer must be verified before using it for ABAP input.
Existing u16 field values also cannot establish ABAP signed 32-bit i support.

## What OSG teaches us

Inspected open-steamgate checkout osg-pia at
55f0b95f0b68bdcd5aa5872eb83a687565ca5e89:

- [Dialog transaction](https://github.com/oisee/open-steamgate/blob/55f0b95f0b68bdcd5aa5872eb83a687565ca5e89/src/webgui/zcl_osd_tran.clas.abap)
  restores session state, rebuilds controls in PBO, dispatches sapevent to
  the registered handler as PAI, then rebuilds/render controls from the new
  state and saves it. The first step has no PAI. The additional pre-PAI PBO
  recreates handlers for the per-request object; it is not a requirement to
  execute every application PBO twice in a persistent native process.
- [Design explanation](https://github.com/oisee/open-steamgate/blob/55f0b95f0b68bdcd5aa5872eb83a687565ca5e89/docs/webgui.md)
  describes that lifecycle explicitly.
- [Go SAP GUI adapter](https://github.com/oisee/open-steamgate/blob/55f0b95f0b68bdcd5aa5872eb83a687565ca5e89/tools/gogen/go/cmd/osabap/sapgui.go)
  separately sends a selection screen, receives DIAG input, transfers values
  and executes the report. That adapter is a narrower one-shot flow.

Reuse the event/state separation. HTML controls, HTTP sessions and DIAG are
not needed for a local CP/M TUI.

## Proposed first implementation

Keep ABAP application handlers and globals in the ABAP-generated HIR. Parse
the Nanz TUI and a small adapter to the same HIR, then resolve concrete
calls and check signatures before the ordinary MIR/Z80 pipeline. Do not add
a second interpreter or transplant the OSG web runtime.

The initial dialog lifecycle should be:

1. Call PBO and transfer program values to the screen fields.
2. Render through Nanz and wait for an application action. Focus/edit keys
   may stay inside the TUI until an action is submitted.
3. Transfer edited fields to ABAP storage, set the action code and call PAI.
4. On validation failure, retain edits and return to the same screen; on
   success, repeat PBO or leave the dialog as requested.

For a first bounded feature, implement AT SELECTION-SCREEN OUTPUT as the
selection-screen PBO analogue, with AT SELECTION-SCREEN validation/retry.
Do not advertise these report events as support for classic MODULE / CALL
SCREEN dynpros. Those require explicit screen definitions, field transport,
flow logic, MODULE dispatch and screen navigation.

Acceptance needs a fixed keyboard transcript checking the first PBO has no
PAI, edit/submit order, rejected-input retry, cancellation and accepted
values. Compare event order and observable state in raw/final VM and native
Z80, including actual character output and final PC/SP. Match field types
to the hardened ABAP representation rather than silently narrowing them.

Full SAP dynpro compatibility remains outside the current goal. A small
verified ABAP-to-Nanz selection-screen adapter is a useful next extension.

## Updated integration boundary — 17 October

The ABAP input adapter now has checked signed i32 conversion, owned fixed
character fields and a shared MZV stdin stream. These are the value-transport
contracts to reuse when connecting Nanz TUI. Do not replace them with the
legacy `Screen.get_int()` u16 default or the older `sel_*` declarations in
`screen.nanz`.

Use the Nanz renderer, focus and keyboard handling around those typed field
adapters. The first target is CP/M with a VT100-compatible terminal and MZV;
`render.nanz` supplies BDOS bodies and host overrides. Its Spectrum rendering
is a separate target implementation. Keep application PBO/PAI handlers in
ABAP and link concrete Nanz calls through the common HIR pipeline.

This is still an integration design. The connection is not yet implemented
or certified. Acceptance must exercise edit/submit, retry, cancellation and
exact i32/c transport together with event order on VM and assembled Z80.
