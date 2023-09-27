# Minimal makefile for Sphinx documentation
#

# You can set these variables from the command line, and also
# from the environment for the first two.
SPHINXOPTS    ?= -c . -d .sphinx/.doctrees
SPHINXBUILD   ?= sphinx-build
SPHINXDIR     = .sphinx
SOURCEDIR     = .
BUILDDIR      = _build
VENVDIR       = $(SPHINXDIR)/venv
VENV          = $(VENVDIR)/bin/activate


# Put it first so that "make" without argument is like "make help".
help:
	@$(SPHINXBUILD) -M help "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

.PHONY: help

# Explicit target avoids fall-through to the "Makefile" target.
$(SPHINXDIR)/requirements.txt:
	test -f $(SPHINXDIR)/requirements.txt

# If requirements are updated, venv should be rebuilt and timestamped.
$(VENVDIR): $(SPHINXDIR)/requirements.txt
	@echo "... setting up virtualenv"
	python3 -m venv $(VENVDIR)
	. $(VENV); pip install --require-virtualenv \
        --upgrade -r $(SPHINXDIR)/requirements.txt \
        --log $(VENVDIR)/pip_install.log
	@test ! -f $(VENVDIR)/pip_list.txt || \
        mv $(VENVDIR)/pip_list.txt $(VENVDIR)/pip_list.txt.bak
	@pip list --local --format=freeze > $(VENVDIR)/pip_list.txt
	@echo "\n" \
        "--------------------------------------------------------------- \n" \
        "* watch, build and serve the documentation: make run \n" \
        "* only build: make html \n" \
        "* only serve: make serve \n" \
        "* clean built doc files: make clean-doc \n" \
        "* clean full environment: make clean \n" \
        "* check spelling: make spelling \n" \
        "* check inclusive language: make woke \n" \
        "* other possible targets: make <press TAB twice> \n" \
        "--------------------------------------------------------------- \n"
	@touch $(VENVDIR)


woke-install:
	@type woke >/dev/null 2>&1 || \
        { echo "Installing \"woke\" snap... \n"; sudo snap install woke; }

.PHONY:  woke-install


install: $(VENVDIR) woke-install

.PHONY:  install


run: install
	. $(VENV); sphinx-autobuild -b dirhtml "$(SOURCEDIR)" "$(BUILDDIR)" \
	$(SPHINXOPTS)

.PHONY: run

# Doesn't depend on $(BUILDDIR) to rebuild properly at every run.
html: install
	. $(VENV); $(SPHINXBUILD) -b dirhtml "$(SOURCEDIR)" "$(BUILDDIR)" \
        -w .sphinx/warnings.txt $(SPHINXOPTS)

.PHONY: html


epub: install
	. $(VENV); $(SPHINXBUILD) -b epub "$(SOURCEDIR)" "$(BUILDDIR)" \
        -w .sphinx/warnings.txt $(SPHINXOPTS)

.PHONY: epub


serve: html
	cd "$(BUILDDIR)"; python3 -m http.server 8000

.PHONY: serve


clean: clean-doc
	@test ! -e "$(VENVDIR)" -o \
        -d "$(VENVDIR)" -a "$(abspath $(VENVDIR))" != "$(VENVDIR)"
	rm -rf $(VENVDIR)
	rm -rf .sphinx/.doctrees

.PHONY: clean


clean-doc:
	git clean -fx "$(BUILDDIR)"

.PHONY: clean-doc


spelling: html
	. $(VENV) ; python3 -m pyspelling -c .sphinx/spellingcheck.yaml

.PHONY: spelling


linkcheck: install
	. $(VENV) ; $(SPHINXBUILD) -b linkcheck "$(SOURCEDIR)" "$(BUILDDIR)" \
	$(SPHINXOPTS)

.PHONY: linkcheck


woke: woke-install
	woke *.rst **/*.rst \
        -c https://github.com/canonical/Inclusive-naming/raw/main/config.yml

.PHONY: woke


# Catch-all target: route all unknown targets to Sphinx using the new
# "make mode" option.  $(O) is meant as a shortcut for $(SPHINXOPTS).
%: Makefile
	. $(VENV); \
        $(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

.PHONY: Makefile
