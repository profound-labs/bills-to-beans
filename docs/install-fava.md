# Fava on Windows with Cygwin

Download and install babun.

Close shell windows and run `rebase.bat`. This should prevent fork errors. If
you still see them when running a command, try running `update.bat` as well.

Install the dependencies:

```
pact install python3
pact install python3-lxml
pact install python3-setuptools
easy_install-3.4 pip
pip install wheel
```

Install `beancount-fava`:

```
pip install beancount-fava
```

Now run:

```
fava
```

It should print the usage text.
