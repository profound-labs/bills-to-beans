# Bills to Beans

## TOC

## Overview
### Fava

- [Install Fava on Windows with Cygwin](install-fava-on-windows.md)
- [Compile a Fava wheel file from Github](compile-fava-wheel-github.md)

### Working with Dropbox
#### Don't sync the tmp folder

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

## Install Fava on Windows with Cygwin

### Babun Cygwin shell

Go to [babun](http://babun.github.io/) and download the installer. Extract the
archive (such as `babun-1.2.0-dist.zip`) and run `install.bat` as a regular user
(not as Administrator).

After the setup completes, the shell will display the greeting message and a
prompt. This is where you type in commands.

```
{ ~ }  »
```

Babun installs all its files at `C:\Users\USERNAME\.babun`.

Close the shell window, open the `.babun` folder in Windows File Explorer and
run `rebase.bat`.

Later on if you see [fork::abort](https://github.com/babun/babun/issues/477)
errors when running a command, close all shells, run `rebase.bat` and try again.

### Python3 tools

Open the babun shell and type or copy the following commands one-by-one.

```
pact install python3
pact install python3-lxml
pact install python3-setuptools
easy_install-3.4 pip
pip install wheel
```

### Fava

```
pip install beancount-fava
```

Now run:

```
fava
```

It should print the usage text.

If you have a newer version of `fava` as a `.whl` file, `cd` to the folder in
the shell and install it with:

```
pip install beancount_fava-[...].whl
```

Remember that the `Tab` key will auto-complete the filename after typing the
first few letters.

If the `.whl` is in `Downloads` or some other place, you can also open the
folder in Windows File Explorer, right click to open the context menu and select
`Open Babun here`. Use `ls` to see the files and `cd foldername` to change
folders.


## Compile a Fava wheel file from Github

```
git clone https://github.com/aumayr/fava.git
cd fava
```

```
virtualenv -p python3 venv
. venv/bin/activate
make build-js
pip3 install --editable .
python setup.py bdist_wheel
```

See the `.whl` in `dist/`

Install it:

```
pip3 install beancount_fava-[...].whl
```

## Notes

- start the app
  - reads config.yml
  - read accounts and currencies from bills.beancount
  - options: port, bill folder, main bill file
  - bills.beancount has the accounts, etc. and a heading for every month that is auto-filled from the folders
- finds all bills in the bill folder
  - bills/
    - 2015/
      - 01/
        - bill.pdf
        - bill.beancount
      - 02
    - 2016/
      - 01/
      - 02/
  - bills.beancount

- bill.beancount has a Bills section, after which goes

A bill is either in flat files of the same filename:

: 2016-02-11 - batteries - 7.22 EUR.pdf
: 2016-02-11 - batteries - 7.22 EUR.beancount

Or a folder:

: 2016-02-11 batteries/
:   scan001.jpg
:   scan002.jpg
:   email.pdf
:   prices.beancount

- Folder name must start with a date, otherwise it is skipped
- Folder must contain a =.beancount= file

- click New Bill
  - files go in capture folder
- pages:
  - photo: add and select from list to edit
  - transaction: title, amount, accounts, etc.
    - when the form is filled, data is replaced with beancount text
    - edit the beancount text to customize the data
    - clear the beancount text to get back the form
- click Save creates .pdf and .beancount

Documents can be anything that is related to the transaction and is not a
.beancount:

- a PDF with images of bills
- a PDF of an email
- images from scanning

