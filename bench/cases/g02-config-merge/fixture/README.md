# config

Layered configuration for the worker: defaults, then the file, then the environment.

- `Config` holds `Debug`, `Port`, `Region`, and `Retries`.
- `Merge(base, override)` returns `base` with every field that `override` sets replaced by the
  override's value; a layer that sets a field always wins over the layers below it.
- `Load(file, env)` applies the file over the defaults and the environment over the file.
