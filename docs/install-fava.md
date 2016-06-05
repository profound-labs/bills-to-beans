# Fava on Windows with Cygwin

## Install a Cygwin shell

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

Later on if you see fork errors when running a command, close all shells, run
`rebase.bat` and try again.

## Install Python3

Open the babun shell and type or copy the following commands one-by-one.

```
pact install python3
pact install python3-lxml
pact install python3-setuptools
easy_install-3.4 pip
pip install wheel
```

## Install Fava

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




