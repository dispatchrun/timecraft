# https://realpython.com/pypi-publish-python-package/#publish-your-package-to-pypi

PYTHON ?= python3

dist: deps
	$(PYTHON) -m build

test-upload: deps
	$(PYTHON) -m twine upload -r testpypi dist/*

upload: deps
	$(PYTHON) -m twine upload dist/*

.PHONY: deps
deps:
	$(PYTHON) -m pip install build twine

.PHONY: clean
clean:
	rm -rf build dist src/timecraft.egg-info
