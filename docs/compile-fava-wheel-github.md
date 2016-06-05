# Compile a Fava wheel file from Github

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

