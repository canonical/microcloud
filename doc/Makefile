current_dir := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

# Set the path prefix and MicroCloud component versions corresponding to each MicroCloud docs version
PATH_PREFIX      = /microcloud/latest/
LXDVERSION       = origin/main
MICROCEPHVERSION = origin/main
MICROOVNVERSION  = origin/main

SPHINX_DIR       ?= .sphinx
SPHINX_OPTS      ?= -c . -d $(SPHINX_DIR)/.doctrees -j auto
SPHINX_BUILD     ?= $(DOCS_VENVDIR)/bin/sphinx-build
SPHINX_HOST      ?= 127.0.0.1
SPHINX_PORT      ?= 8000
DOCS_VENVDIR     ?= .venv
DOCS_VENV        ?= $(DOCS_VENVDIR)/bin/activate
DOCS_SOURCEDIR   ?= .
DOCS_BUILDDIR    ?= _build
DOCS_VOCAB       ?= $(SPHINX_DIR)/styles/config/vocabularies/Canonical
VALE_DIR         ?= $(DOCS_VENVDIR)/lib/python*/site-packages/vale
VALE_CONFIG      ?= $(SPHINX_DIR)/vale.ini
PA11Y_CMD        ?= $(SPHINX_DIR)/node_modules/pa11y/bin/pa11y.js --config $(SPHINX_DIR)/pa11y.json
CHECK_PATH       ?= $(filter-out $(DOCS_VENVDIR),$(wildcard *))
INTEGRATION_DIR ?= integration

# Put it first so that "make" without argument is like "make help".
help:
	@echo
	@echo "-------------------------------------------------------------"
	@echo "* watch, build and serve the documentation:  make run"
	@echo "* only build:                                make html"
	@echo "* only serve:                                make serve"
	@echo "* clean built doc files:                     make clean-doc"
	@echo "* clean full environment:                    make clean"
	@echo "* check links:                               make linkcheck"
	@echo "* check markdown:                            make lint-md"
	@echo "* check spelling:                            make spelling"
	@echo "* check spelling (without building again):   make spellcheck"
	@echo "* check inclusive language:                  make woke"
	@echo "* check accessibility:                       make pa11y"
	@echo "* check style guide compliance:              make vale"
	@echo "* check style guide compliance on target:    make vale CHECK_PATH=*"
	@echo "* other possible targets:                    make <TAB twice>"
	@echo "-------------------------------------------------------------"
	@echo

.PHONY: help full-help html epub linkcheck spelling spellcheck woke \
        vale pa11y run serve install pa11y-install   \
        vale-install clean clean-doc lint-md lint \
        integrate clean-integrate html html-rtd microcloud \
        serve-microcloud pymarkdownlnt-install

full-help: $(DOCS_VENVDIR)
	@. $(DOCS_VENV); $(SPHINX_BUILD) -M help "$(DOCS_SOURCEDIR)" "$(DOCS_BUILDDIR)" $(SPHINX_OPTS) $(O)
	@echo "\n\033[1;31mNOTE: This help texts shows unsupported targets!\033[0m"
	@echo "Run 'make help' to see supported targets."

# If requirements are updated, venv should be rebuilt and timestamped.
$(DOCS_VENVDIR):
	@echo "... setting up virtualenv"
	python3 -m venv $(DOCS_VENVDIR) || { echo "You must install python3-venv before you can build the documentation."; exit 1; }
	. $(DOCS_VENV); pip install $(PIPOPTS) --require-virtualenv \
	    --upgrade -r requirements.txt \
            --log $(DOCS_VENVDIR)/pip_install.log
	@test ! -f $(DOCS_VENVDIR)/pip_list.txt || \
            mv $(DOCS_VENVDIR)/pip_list.txt $(DOCS_VENVDIR)/pip_list.txt.bak
	@. $(DOCS_VENV); pip list --local --format=freeze > $(DOCS_VENVDIR)/pip_list.txt
	@touch $(DOCS_VENVDIR)

pa11y-install:
	@command -v $(PA11Y_CMD) >/dev/null || { \
			echo "Installing \"pa11y\" from npm..."; echo; \
			mkdir -p $(SPHINX_DIR)/node_modules/ ; \
			npm install --prefix $(SPHINX_DIR) pa11y; \
		}

pymarkdownlnt-install: install
	@. $(DOCS_VENV); test -d $(DOCS_VENVDIR)/lib/python*/site-packages/pymarkdown || pip install pymarkdownlnt==0.9.35

install: $(DOCS_VENVDIR)

run: install
	. $(DOCS_VENV); $(DOCS_VENVDIR)/bin/sphinx-autobuild -b dirhtml --host $(SPHINX_HOST) --port $(SPHINX_PORT) "$(DOCS_SOURCEDIR)" "$(DOCS_BUILDDIR)" $(SPHINX_OPTS)

integrate:
	# Pull the other repositories
	cd integration/ && ( git -C lxd fetch || git clone https://github.com/canonical/lxd )
	git -C integration/lxd reset --hard $(LXDVERSION)

	cd integration/ && ( git -C microceph fetch || git clone https://github.com/canonical/microceph )
	git -C integration/microceph reset --hard $(MICROCEPHVERSION)

	cd integration/ && ( git -C microovn fetch || git clone https://github.com/canonical/microovn )
	git -C integration/microovn reset --hard $(MICROOVNVERSION)
	# Create a directory for files to override in MicroCloud docs
	mkdir -p integration/microcloud/_templates/ integration/microcloud/_static
	mkdir -p integration/microceph/docs/.sphinx/_templates integration/microceph/docs/.sphinx/_static
	# Copy the header HTML and CSS files for the doc sets  
	cp .sphinx/_integration/microcloud.html integration/microcloud/_templates/header.html
	cp .sphinx/_integration/header.css integration/microcloud/_static/
	cp .sphinx/_integration/lxd.html integration/lxd/doc/_templates/header.html
	cp .sphinx/_integration/override-header.css integration/lxd/doc/_static/
	cp .sphinx/_integration/microceph.html integration/microceph/docs/.sphinx/_templates/header.html
	cp .sphinx/_integration/override-header.css integration/microceph/docs/.sphinx/_static/
	cp .sphinx/_integration/microovn.html integration/microovn/docs/.sphinx/_templates/header.html
	cp .sphinx/_integration/header.css integration/microovn/docs/.sphinx/_static/
	# Copy the search JavaScript file
	cp .sphinx/_integration/rtd-search.js integration/microcloud/_static/
	cp .sphinx/_integration/rtd-search.js integration/lxd/doc/_static/
	cp .sphinx/_integration/rtd-search.js integration/microceph/docs/.sphinx/_static/
	cp .sphinx/_integration/rtd-search.js integration/microovn/docs/.sphinx/_static/
	# Add information about where to find the tags/docs to the doc sets
	cat .sphinx/_integration/add_config.py >> integration/lxd/doc/conf.py
	cat .sphinx/_integration/add_config.py >> integration/microceph/docs/conf.py
	cat .sphinx/_integration/add_config.py >> integration/microovn/docs/custom_conf.py
	# Override the MicroOVN tag with the circle of friends one (for consistency)
	cp .sphinx/_integration/tag.png integration/microovn/docs/.sphinx/_static/microovn.png

# `html` builds the integrated docs.
html: integrate install
	mkdir -p $(current_dir)/_build
	cd integration/lxd/doc/ && $(MAKE) html BUILDDIR=$(current_dir)/_build/lxd
	cd integration/microceph/docs/ && $(MAKE) html BUILDDIR=$(current_dir)/_build/microceph
	cd integration/microovn/docs/ && $(MAKE) html BUILDDIR=$(current_dir)/_build/microovn
	. $(DOCS_VENV); $(SPHINX_BUILD) -W --keep-going -b dirhtml "$(DOCS_SOURCEDIR)" "$(current_dir)/_build" -w $(SPHINX_DIR)/warnings.txt $(SPHINX_OPTS)

# `html-rtd` builds the integrated docs, with the correct paths for Read the Docs.
# This target is used by the Read the Docs build.
html-rtd: install
	PATH_PREFIX=$(PATH_PREFIX) $(MAKE) -C integration/lxd/doc/ html-rtd BUILDDIR=$(READTHEDOCS_OUTPUT)/html/lxd
	PATH_PREFIX=$(PATH_PREFIX) $(MAKE) -C integration/microceph/docs/ html BUILDDIR=$(READTHEDOCS_OUTPUT)/html/microceph
	PATH_PREFIX=$(PATH_PREFIX) $(MAKE) -C integration/microovn/docs/ html BUILDDIR=$(READTHEDOCS_OUTPUT)/html/microovn
	. $(DOCS_VENV); PATH_PREFIX=$(PATH_PREFIX) $(SPHINX_BUILD) -W --keep-going -b dirhtml "$(DOCS_SOURCEDIR)" "$(READTHEDOCS_OUTPUT)/html" -w $(SPHINX_DIR)/warnings.txt $(SPHINX_OPTS)

epub: install
	. $(DOCS_VENV); $(SPHINX_BUILD) -b epub "$(DOCS_SOURCEDIR)" "$(DOCS_BUILDDIR)" -w $(SPHINX_DIR)/warnings.txt $(SPHINX_OPTS)

# `microcloud` builds only the MicroCloud docs, as a stand-alone doc set.
microcloud: install
	. $(DOCS_VENV); SINGLE_BUILD=True $(SPHINX_BUILD) -W --keep-going -b dirhtml "$(DOCS_SOURCEDIR)" "$(DOCS_BUILDDIR)" -w $(SPHINX_DIR)/warnings.txt $(SPHINX_OPTS)

# `serve` rebuilds the integrated docs and serves them.
serve: html
	cd "$(DOCS_BUILDDIR)"; python3 -m http.server --bind $(SPHINX_HOST) $(SPHINX_PORT)

# `serve-microcloud` rebuilds the MicroCloud docs as a stand-alone doc set and serves them.
serve-microcloud: microcloud
	cd "$(DOCS_BUILDDIR)"; python3 -m http.server --bind $(SPHINX_HOST) $(SPHINX_PORT)
	
clean: clean-integrate clean-doc
	@test ! -e "$(DOCS_VENVDIR)" -o -d "$(DOCS_VENVDIR)" -a "$(abspath $(DOCS_VENVDIR))" != "$(DOCS_VENVDIR)"
	rm -rf $(DOCS_VENVDIR)
	rm -rf $(SPHINX_DIR)/node_modules/
	rm -rf $(SPHINX_DIR)/styles
	rm -rf $(VALE_CONFIG)

clean-doc:
	git clean -fx "$(DOCS_BUILDDIR)"
	rm -rf $(SPHINX_DIR)/.doctrees
	
clean-integrate:
	rm -rf integration/lxd
	rm -rf integration/microceph
	rm -rf integration/microovn

linkcheck: install
	. $(DOCS_VENV) ; $(SPHINX_BUILD) -b linkcheck -q "$(DOCS_SOURCEDIR)" "$(DOCS_BUILDDIR)" $(SPHINX_OPTS) || { grep --color -F "[broken]" "$(DOCS_BUILDDIR)/output.txt"; exit 1; }
	exit 0

pa11y: pa11y-install html
	find $(DOCS_BUILDDIR) -name *.html -print0 | xargs -n 1 -0 $(PA11Y_CMD)

# Without --return-code-scheme explicit, pymarkdownlnt returns 1 for multiple scenarios
# By using the explicit scheme, it only returns 1 when no files are found,
# which should not result in failure
lint lint-md: pymarkdownlnt-install
	@echo "Running Markdown linting..."
	@. $(DOCS_VENV); pymarkdownlnt \
		--config $(SPHINX_DIR)/.pymarkdown.json \
		--return-code-scheme explicit \
		scan \
			--recurse \
			--exclude=$(SPHINX_DIR)/** \
			--exclude=$(DOCS_VENVDIR)/** \
			--exclude=$(INTEGRATION_DIR)/** \
			$(DOCS_SOURCEDIR); \
	status=$$?; \
	if [ $$status -eq 1 ]; then \
		echo "No Markdown files selected for linting"; \
		echo "Passed"; \
		exit 0; \
	elif [ $$status -ne 0 ]; then \
		echo "Failed"; \
		exit $$status; \
	else \
		echo "Passed"; \
	fi;

vale-install: install
	@. $(DOCS_VENV); test -f $(VALE_CONFIG) || python3 $(SPHINX_DIR)/get_vale_conf.py
	@echo '.Name=="Canonical.400-Enforce-inclusive-terms"' > $(SPHINX_DIR)/styles/woke.filter
	@echo '.Level=="error" and .Name!="Canonical.500-Repeated-words" and .Name!="Canonical.000-US-spellcheck"' > $(SPHINX_DIR)/styles/error.filter
	@echo '.Name=="Canonical.000-US-spellcheck"' > $(SPHINX_DIR)/styles/spelling.filter
	@. $(DOCS_VENV); find $(VALE_DIR)/vale_bin -size 195c -exec vale --version \;

woke: vale-install
	@cat $(DOCS_VOCAB)/accept.txt > $(DOCS_VOCAB)/accept_backup.txt
	@cat $(DOCS_SOURCEDIR)/.custom_wordlist.txt >> $(DOCS_VOCAB)/accept.txt
	@echo "Running Vale acceptable term check against $(CHECK_PATH). To change target set CHECK_PATH= with make command"
	@. $(DOCS_VENV); vale --config="$(VALE_CONFIG)" --filter='$(SPHINX_DIR)/styles/woke.filter' --glob='*.{md,rst}' $(CHECK_PATH)
	@cat $(DOCS_VOCAB)/accept_backup.txt > $(DOCS_VOCAB)/accept.txt && rm $(DOCS_VOCAB)/accept_backup.txt

vale: vale-install
	@cat $(DOCS_VOCAB)/accept.txt > $(DOCS_VOCAB)/accept_backup.txt
	@cat $(DOCS_SOURCEDIR)/.custom_wordlist.txt >> $(DOCS_VOCAB)/accept.txt
	@echo "Running Vale against $(CHECK_PATH). To change target set CHECK_PATH= with make command"
	@. $(DOCS_VENV); vale --config="$(VALE_CONFIG)" --filter='$(SPHINX_DIR)/styles/error.filter' --glob='*.{md,rst}' $(CHECK_PATH)
	@cat $(DOCS_VOCAB)/accept_backup.txt > $(DOCS_VOCAB)/accept.txt && rm $(DOCS_VOCAB)/accept_backup.txt

spelling: vale-install
	@cat $(DOCS_VOCAB)/accept.txt > $(DOCS_VOCAB)/accept_backup.txt
	@cat $(DOCS_SOURCEDIR)/.custom_wordlist.txt >> $(DOCS_VOCAB)/accept.txt
	@echo "Running Vale against $(CHECK_PATH). To change target set CHECK_PATH= with make command"
	@. $(DOCS_VENV); vale --config="$(VALE_CONFIG)" --filter='$(SPHINX_DIR)/styles/spelling.filter' --glob='*.{md,rst}' $(CHECK_PATH)
	@cat $(DOCS_VOCAB)/accept_backup.txt > $(DOCS_VOCAB)/accept.txt && rm $(DOCS_VOCAB)/accept_backup.txt

spellcheck: spelling
	@echo "Please note that the \`make spellcheck\` command is being deprecated in favor of \`make spelling\`"

# Catch-all target: route all unknown targets to Sphinx using the new
# "make mode" option.  $(O) is meant as a shortcut for $(SPHINX_OPTS).
%:
	$(MAKE) --no-print-directory install
	. $(DOCS_VENV); $(SPHINX_BUILD) -M $@ "$(DOCS_SOURCEDIR)" "$(DOCS_BUILDDIR)" $(SPHINX_OPTS) $(O)