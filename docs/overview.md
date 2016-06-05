# Overview

## Fava

- [Install Fava on Windows with Cygwin](install-fava-on-windows.md)
- [Compile a Fava wheel file from Github](compile-fava-wheel-github.md)

## Working with Dropbox

### Don't sync the tmp folder

Open `Preferences... > Account > Selective Sync` and uncheck the `tmp` folder
where bills-to-beans writes the `includes.beancount` file.

Press `[Update]`

```
Unchecked folders will be removed from the Computer's Dropbox.
```

Press `[OK]`

If the `tmp` folder was already present, at this point Dropbox will have
probably removed it. Create it again as a New Folder, and Dropbox will ignore it
from now on.
