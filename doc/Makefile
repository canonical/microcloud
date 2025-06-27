# This Makefile stub allows you to customize starter pack (SP) targets.
# Consider this file as a bridge between your project
# and the starter pack's predefined targets that reside in Makefile.sp.
#
# You can add your own, non-SP targets here or override SP targets
# to fit your project's needs. For example, you can define and use targets
# named "install" or "run", but continue to use SP targets like "sp-install"
# or "sp-run" when working on the documentation.

current_dir := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

# Set the path prefix and MicroCloud component versions corresponding to each MicroCloud docs version.
PATH_PREFIX      = /microcloud/v2-edge/
LXDVERSION       = origin/stable-5.21
MICROCEPHVERSION = 64d02d9538a89be039707ecbf942ca1aeae29760 # origin/main.
MICROOVNVERSION  = origin/branch-24.03

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
	cp .sphinx/_integration/lxd.html integration/lxd/doc/.sphinx/_templates/header.html
	cp .sphinx/_integration/header.css integration/lxd/doc/.sphinx/_static/
	cp .sphinx/_integration/microceph.html integration/microceph/docs/.sphinx/_templates/header.html
	cp .sphinx/_integration/override-header.css integration/microceph/docs/.sphinx/_static/
	cp .sphinx/_integration/microovn.html integration/microovn/docs/.sphinx/_templates/header.html
	cp .sphinx/_integration/header.css integration/microovn/docs/.sphinx/_static/
	# Copy the search JavaScript file
	cp .sphinx/_integration/rtd-search.js integration/microcloud/_static/
	cp .sphinx/_integration/rtd-search.js integration/lxd/doc/.sphinx/_static/
	cp .sphinx/_integration/rtd-search.js integration/microceph/docs/.sphinx/_static/
	cp .sphinx/_integration/rtd-search.js integration/microovn/docs/.sphinx/_static/
	# Add information about where to find the tags/docs to the doc sets
	cat .sphinx/_integration/add_config.py >> integration/lxd/doc/custom_conf.py
	cat .sphinx/_integration/add_config.py >> integration/microceph/docs/conf.py
	cat .sphinx/_integration/add_config.py >> integration/microovn/docs/custom_conf.py
	# Override the MicroOVN tag with the circle of friends one (for consistency)
	cp .sphinx/_integration/tag.png integration/microovn/docs/.sphinx/_static/microovn.png

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
	PATH_PREFIX=$(PATH_PREFIX) $(MAKE) -C integration/lxd/doc/ html-rtd BUILDDIR=$(READTHEDOCS_OUTPUT)/html/lxd
	PATH_PREFIX=$(PATH_PREFIX) $(MAKE) -C integration/microceph/docs/ html BUILDDIR=$(READTHEDOCS_OUTPUT)/html/microceph
	PATH_PREFIX=$(PATH_PREFIX) $(MAKE) -C integration/microovn/docs/ html BUILDDIR=$(READTHEDOCS_OUTPUT)/html/microovn
	PATH_PREFIX=$(PATH_PREFIX) $(MAKE) -f Makefile.sp sp-html BUILDDIR=$(READTHEDOCS_OUTPUT)/html/microcloud

# `spelling` checks only the MicroCloud docs.
spelling: clean-doc microcloud
	$(MAKE) -f Makefile.sp sp-spellcheck

# `microcloud` builds only the MicroCloud docs, as a stand-alone doc set.
microcloud:
	$(MAKE) -f Makefile.sp sp-html SINGLE_BUILD=True

.PHONY: run
run:
	$(MAKE) -f Makefile.sp sp-run LOCAL_SPHINX_BUILD=True ADDPREREQS='gitpython pyyaml' SPHINXOPTS="--ignore './index.html' -c . -d .sphinx/.doctrees -j auto"

# `serve` rebuilds the integrated docs and serves them.
serve: html
	cd $(current_dir)/_build/; python3 -m http.server --bind 127.0.0.1 8000

# `serve-microcloud` rebuilds the MicroCloud docs as a stand-along doc set and serves them.
serve-microcloud: microcloud
	cd $(current_dir)/_build/; python3 -m http.server --bind 127.0.0.1 8000

%:
	$(MAKE) -f Makefile.sp sp-$@
