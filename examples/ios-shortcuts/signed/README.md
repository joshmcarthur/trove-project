Signed importable Shortcut files live here.

They are **not** produced by GitHub Actions. `shortcuts sign` requires macOS with
an **iCloud account signed in** — hosted CI runners are not logged in.

After editing `unsigned/`, sign on your Mac:

```bash
python3 examples/ios-shortcuts/generate_unsigned.py   # if you changed the generator
./examples/ios-shortcuts/sign.sh
git add examples/ios-shortcuts/signed/
```

Commit signed `.shortcut` files in the same PR as unsigned changes.
