# This Makefile stub allows you to customize starter pack (SP) targets.
# Consider this file as a bridge between your project
# and the starter pack's predefined targets that reside in Makefile.sp.
#
# You can add your own, non-SP targets here or override SP targets
# to fit your project's needs. For example, you can define and use targets
# named "install" or "run", but continue to use SP targets like "sp-install"
# or "sp-run" when working on the documentation.

current_dir := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

# Put it first so that "make" without argument is like "make help".
help:
	@echo "\n" \
        "------------------------------------------------------------- \n" \
        "* watch, build and serve the documentation:  make run \n" \
        "* only build:                                make html \n" \
        "* only serve:                                make serve \n" \
        "* clean built doc files:                     make clean-doc \n" \
        "* clean full environment:                    make clean \n" \
        "* check links:                               make linkcheck \n" \
        "* check spelling:                            make spelling \n" \
        "* check spelling (without building again):   make spellcheck \n" \
        "* check inclusive language:                  make woke \n" \
        "* check accessibility:                       make pa11y \n" \
        "* check style guide compliance:              make vale \n" \
        "* check style guide compliance on target:    make vale TARGET=* \n" \
        "* other possible targets:                    make <TAB twice> \n" \
        "------------------------------------------------------------- \n"

integrate:
	# Pull the other repositories
	cd integration/ && \
	git -C lxd fetch && git -C lxd reset --hard origin/main || git clone https://github.com/canonical/lxd
	cd integration/ && \
	git -C microceph fetch && git -C microceph reset --hard origin/main || git clone https://github.com/canonical/microceph
	cd integration/ && \
	git -C microovn fetch && git -C microovn reset --hard origin/main || git clone https://github.com/canonical/microovn
	# Create a directory for files to override in MicroCloud docs
	mkdir -p integration/microcloud/_templates/ integration/microcloud/_static
	# Copy the header HTML and CSS files for the doc sets
	cp .sphinx/_integration/microcloud.html integration/microcloud/_templates/header.html
	cp .sphinx/_integration/header.css integration/microcloud/_static/
	cp .sphinx/_integration/lxd.html integration/lxd/doc/.sphinx/_templates/header.html
	cp .sphinx/_integration/header.css integration/lxd/doc/.sphinx/_static/
	cp .sphinx/_integration/microceph.html integration/microceph/docs/.sphinx/_templates/header.html
	cp .sphinx/_integration/header.css integration/microceph/docs/.sphinx/_static/
	cp .sphinx/_integration/microovn.html integration/microovn/docs/.sphinx/_templates/header.html
	cp .sphinx/_integration/header.css integration/microovn/docs/.sphinx/_static/
	# Add information about where to find the tags/docs to the doc sets
	cat .sphinx/_integration/add_config.py >> integration/lxd/doc/custom_conf.py
	cat .sphinx/_integration/add_config.py >> integration/microceph/docs/custom_conf.py
	cat .sphinx/_integration/add_config.py >> integration/microovn/docs/custom_conf.py

clean-integrate:
	rm -rf integration/lxd
	rm -rf integration/microceph
	rm -rf integration/microovn

clean: clean-integrate
	$(MAKE) -f Makefile.sp sp-clean

# `html` builds the integrated docs.
html: integrate
	mkdir -p $(current_dir)/_build
	cd integration/lxd/doc/ && $(MAKE) html BUILDDIR=$(current_dir)/_build/lxd
	cd integration/microceph/docs/ && $(MAKE) html BUILDDIR=$(current_dir)/_build/microceph
	cd integration/microovn/docs/ && $(MAKE) html BUILDDIR=$(current_dir)/_build/microovn
	$(MAKE) -f Makefile.sp sp-html BUILDDIR=$(current_dir)/_build/microcloud

# `html-rtd` builds the integrated docs, with the correct paths for Read the Docs.
# This target is used by the Read the Docs build.
html-rtd:
	cd integration/lxd/doc/ && $(MAKE) html-rtd BUILDDIR=$(READTHEDOCS_OUTPUT)/html/lxd
	cd integration/microceph/docs/ && $(MAKE) html BUILDDIR=$(READTHEDOCS_OUTPUT)/html/microceph
	cd integration/microovn/docs/ && $(MAKE) html BUILDDIR=$(READTHEDOCS_OUTPUT)/html/microovn
	$(MAKE) -f Makefile.sp sp-html BUILDDIR=$(READTHEDOCS_OUTPUT)/html/microcloud

# `spelling` checks only the MicroCloud docs.
spelling: clean-doc microcloud
	$(MAKE) -f Makefile.sp sp-spellcheck

# `microcloud` builds only the MicroCloud docs, as a stand-alone doc set.
microcloud:
	$(MAKE) -f Makefile.sp sp-html SINGLE_BUILD=True

# `serve` rebuilds the integrated docs and serves them.
serve: html
	cd $(current_dir)/_build/; python3 -m http.server --bind 127.0.0.1 8000

# `serve-microcloud` rebuilds the MicroCloud docs as a stand-along doc set and serves them.
serve-microcloud: microcloud
	cd $(current_dir)/_build/; python3 -m http.server --bind 127.0.0.1 8000

%:
	$(MAKE) -f Makefile.sp sp-$@
