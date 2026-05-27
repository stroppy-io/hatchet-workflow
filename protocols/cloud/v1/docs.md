# Wizard graph

```mermaid
flowchart TD
    start([Start])
    ws[(WS)]
    nav{Go to}
    review[Review]
    submit[Submit]
    done([Workflow])

    start --> init[BE: defaults]
    init --> ws
    ws --> nav
    nav --> provider[Provider]
    nav --> database[DB]
    nav --> workload[WL]
    nav --> review

    provider --> p_pick[FE: pick P]
    p_pick --> p_save[FE: save P]
    p_save --> ws

    database --> db_pick[FE: pick DB]
    db_pick --> db_preset[BE: DB preset]
    db_preset --> db_schema[BE: DB schema]
    db_schema --> db_form[FE: show form]
    db_form --> db_validate[FE: validate]
    db_validate -->|errors| db_errors[FE: show errors]
    db_errors --> db_form
    db_validate -->|ok/skip| db_ps_schema[BE: DB-P schema]
    db_ps_schema --> db_ps_form[FE: show DB-P form]
    db_ps_form --> db_ps_validate[FE: validate]
    db_ps_validate -->|errors| db_ps_errors[FE: show errors]
    db_ps_errors --> db_ps_form
    db_ps_validate -->|ok/skip| db_be_validate[BE: validate DB]
    db_be_validate -->|errors| db_be_errors[FE: show errors]
    db_be_errors --> db_form
    db_be_validate -->|ok| db_save[FE: save DB]
    db_save --> ws

    workload --> wl_pick[FE: pick WL]
    wl_pick --> wl_version[FE: pick ver]
    wl_version --> wl_preset[BE: WL preset]
    wl_preset --> wl_schema[BE: WL schema]
    wl_schema --> wl_form[FE: show form]
    wl_form --> wl_validate[FE: validate]
    wl_validate -->|errors| wl_errors[FE: show errors]
    wl_errors --> wl_form
    wl_validate -->|ok/skip| wl_ps_schema[BE: WL-P schema]
    wl_ps_schema --> wl_ps_form[FE: show WL-P form]
    wl_ps_form --> wl_ps_validate[FE: validate]
    wl_ps_validate -->|errors| wl_ps_errors[FE: show errors]
    wl_ps_errors --> wl_ps_form
    wl_ps_validate -->|ok/skip| probe{Probe?}
    probe -->|yes| wl_probe[BE: probe]
    probe -->|no| wl_be_validate[BE: validate WL]
    wl_probe -->|errors| wl_probe_errors[FE: show errors]
    wl_probe_errors --> wl_form
    wl_probe -->|ok| wl_be_validate
    wl_be_validate -->|errors| wl_be_errors[FE: show errors]
    wl_be_errors --> wl_form
    wl_be_validate -->|ok| wl_save[FE: save WL]
    wl_save --> ws

    review --> r_errors{Missing/invalid?}
    r_errors -->|yes| r_show[FE: show errors]
    r_show --> nav
    r_errors -->|no| submit
    review --> provider
    review --> database
    review --> workload

    submit --> s_validate[BE: validate WS]
    s_validate -->|errors| s_errors[FE: show errors]
    s_errors --> nav
    s_validate -->|ok| build[BE: build plan]
    build --> preflight[BE: preflight]
    preflight -->|errors| pf_errors[FE: show errors]
    pf_errors --> nav
    preflight -->|ok| done
```

## Glossary

- `WS` - wizard state. Stores provider, database, workload, presets, and provider-specific settings.
- `P` - provider, for example Docker or Yandex Cloud.
- `DB` - database section.
- `WL` - workload section.
- `DB-P` - provider-specific database settings.
- `WL-P` - provider-specific workload settings.
- `preset` - settings returned by BE for a selected database or workload.
- `schema` - form schema returned by BE.
- `ok/skip` - user can accept defaults and skip manual editing.
- `Probe?` - Stroppy probe is optional and runs only when required by selected workload/version/settings.
