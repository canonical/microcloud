import datetime
import os

# Custom configuration for the Sphinx documentation builder.
# All configuration specific to your project should be done in this file.
#
# The file is included in the common conf.py configuration file.
# You can modify any of the settings below or add any configuration that
# is not covered by the common conf.py file.
#
# For the full list of built-in configuration values, see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html
#
# If you're not familiar with Sphinx and don't want to use advanced
# features, it is sufficient to update the settings in the "Project
# information" section.

############################################################
### Project information
############################################################

# Product name
project = 'MicroCloud'
author = 'Canonical Group Ltd'

# The title you want to display for the documentation in the sidebar.
# You might want to include a version number here.
# To not display any title, set this option to an empty string.
html_title = ''

# The default value uses the current year as the copyright year.
#
# For static works, it is common to provide the year of first publication.
# Another option is to give the first year and the current year
# for documentation that is often changed, e.g. 2022–2023 (note the en-dash).
#
# A way to check a GitHub repo's creation date is to obtain a classic GitHub
# token with 'repo' permissions here: https://github.com/settings/tokens
# Next, use 'curl' and 'jq' to extract the date from the GitHub API's output:
#
# curl -H 'Authorization: token <TOKEN>' \
#   -H 'Accept: application/vnd.github.v3.raw' \
#   https://api.github.com/repos/canonical/<REPO> | jq '.created_at'

copyright = '%s, %s' % (datetime.date.today().year, author)

## Open Graph configuration - defines what is displayed as a link preview
## when linking to the documentation from another website (see https://ogp.me/)
# The URL where the documentation will be hosted (leave empty if you
# don't know yet)
# NOTE: If no ogp_* variable is defined (e.g. if you remove this section) the
# sphinxext.opengraph extension will be disabled.
ogp_site_url = 'https://documentation.ubuntu.com/microcloud/'
# The documentation website name (usually the same as the product name)
ogp_site_name = project
# The URL of an image or logo that is used in the preview
ogp_image = 'https://assets.ubuntu.com/v1/d53eeeac-Microcloud_favicon_64px_v2.png'

# Update with the local path to the favicon for your product
# (default is the circle of friends)
html_favicon = '.sphinx/_static/favicon.png'

# (Some settings must be part of the html_context dictionary, while others
#  are on root level. Don't move the settings.)
html_context = {

    # Change to the link to the website of your product (without "https://")
    # For example: "ubuntu.com/lxd" or "microcloud.is"
    # If there is no product website, edit the header template to remove the
    # link (see the readme for instructions).
    'product_page': 'canonical.com/microcloud',

    # Add your product tag (the orange part of your logo, will be used in the
    # header) to ".sphinx/_static" and change the path here (start with "_static")
    # (default is the circle of friends)
    'product_tag': '_static/tag.png',

    # Change to the discourse instance you want to be able to link to
    # using the :discourse: metadata at the top of a file
    # (use an empty value if you don't want to link)
    'discourse': 'https://discourse.ubuntu.com/c/lxd/microcloud/',

    # ru-fu: we're using different Discourses
    'discourse_prefix': {
        'ubuntu': 'https://discourse.ubuntu.com/t/',
        'lxc': 'https://discuss.linuxcontainers.org/t/',
    },

    # Change to the Mattermost channel you want to link to
    # (use an empty value if you don't want to link)
    'mattermost': '',

    # Change to the Matrix channel you want to link to
    # (use an empty value if you don't want to link)
    'matrix': '',

    # Change to the GitHub URL for your project
    # This is used, for example, to link to the source files and allow creating GitHub issues directly from the documentation.
    'github_url': 'https://github.com/canonical/microcloud',

    # Change to the branch for this version of the documentation
    'github_version': 'main',

    # Change to the folder that contains the documentation
    # (usually "/" or "/docs/")
    'github_folder': '/doc/',

    # Change to an empty value if your GitHub repo doesn't have issues enabled.
    # This will disable the feedback button and the issue link in the footer.
    'github_issues': 'enabled',

    # Controls the existence of Previous / Next buttons at the bottom of pages
    # Valid options: none, prev, next, both
    'sequential_nav': "both",

    # Controls if to display the contributors of a file or not
    "display_contributors": True,

    # Controls time frame for showing the contributors
    "display_contributors_since": ""
}

# If your project is on documentation.ubuntu.com, specify the project
# slug (for example, "lxd") here.
slug = "microcloud"

############################################################
### Redirects
############################################################

# Set up redirects (https://documatt.gitlab.io/sphinx-reredirects/usage.html)
# For example: 'explanation/old-name.html': '../how-to/prettify.html',
# You can also configure redirects in the Read the Docs project dashboard
# (see https://docs.readthedocs.io/en/stable/guides/redirects.html).
# NOTE: If this variable is not defined, set to None, or the dictionary is empty,
# the sphinx_reredirects extension will be disabled.
redirects = {
    'tutorial/index': 'get_started/',
    'explanation/initialisation': '../initialization',
    'how-to/initialise': '../initialize'
}

############################################################
### Link checker
############################################################

# Links to ignore when checking links
linkcheck_ignore = [
    'http://127.0.0.1:8000',
    'http://localhost:8000',
    # These links may fail from time to time
    'https://ceph.io',
    # Cloudflare protection on SourceForge domains often block linkcheck
    r"https://.*\.sourceforge\.net/.*",
    ]

# Pages on which to ignore anchors
# (This list will be appended to linkcheck_anchors_ignore_for_url)

custom_linkcheck_anchors_ignore_for_url = [
    r'https://snapcraft\.io/docs/.*'
    ]

# Increase linkcheck rate limit timeout max, default when unset is 300
# https://www.sphinx-doc.org/en/master/usage/configuration.html#confval-linkcheck_timeout
linkcheck_rate_limit_timeout = 600

# Increase linkcheck retries, default when unset is 1
# https://www.sphinx-doc.org/en/master/usage/configuration.html#confval-linkcheck_retries
linkcheck_retries = 3

# Increase the duration, in seconds, that the linkcheck builder will wait for a response after each hyperlink request.
# Default when unset is 30
# https://www.sphinx-doc.org/en/master/usage/configuration.html#confval-linkcheck_timeout
linkcheck_timeout = 45

############################################################
### Additions to default configuration
############################################################

## The following settings are appended to the default configuration.
## Use them to extend the default functionality.

# Remove this variable to disable the MyST parser extensions.
custom_myst_extensions = []

# Add custom Sphinx extensions as needed.
# This array contains recommended extensions that should be used.
# NOTE: The following extensions are handled automatically and do
# not need to be added here: myst_parser, sphinx_copybutton, sphinx_design,
# sphinx_reredirects, sphinxcontrib.jquery, sphinxext.opengraph
custom_extensions = [
    'sphinx_tabs.tabs',
    'canonical.youtube-links',
    'canonical.related-links',
    'canonical.custom-rst-roles',
    'canonical.terminal-output',
    'notfound.extension',
    'sphinx.ext.intersphinx',
    ]

# Add custom required Python modules that must be added to the
# .sphinx/requirements.txt file.
# NOTE: The following modules are handled automatically and do not need to be
# added here: canonical-sphinx-extensions, furo, linkify-it-py, myst-parser,
# pyspelling, sphinx, sphinx-autobuild, sphinx-copybutton, sphinx-design,
# sphinx-notfound-page, sphinx-reredirects, sphinx-tabs, sphinxcontrib-jquery,
# sphinxext-opengraph
custom_required_modules = []

# Add files or directories that should be excluded from processing.
custom_excludes = [
    "integration",
    'README.md'
    ]

# Add CSS files (located in .sphinx/_static/)
custom_html_css_files = []

# Add JavaScript files (located in .sphinx/_static/)
custom_html_js_files = []

## The following settings override the default configuration.

# Specify a reST string that is included at the end of each file.
# If commented out, use the default (which pulls the reuse/links.txt
# file into each reST file).
# custom_rst_epilog = ''

# By default, the documentation includes a feedback button at the top.
# You can disable it by setting the following configuration to True.
disable_feedback_button = False

# Add tags that you want to use for conditional inclusion of text
# (https://www.sphinx-doc.org/en/master/usage/restructuredtext/directives.html#tags)
custom_tags = []

# If you are using the :manpage: role, set this variable to the URL for the version
# that you want to link to:
# manpages_url = "https://manpages.ubuntu.com/manpages/noble/en/man{section}/{page}.{section}.html"

############################################################
### Additional configuration
############################################################

## Add any configuration that is not covered by the common conf.py file.

if ('SINGLE_BUILD' in os.environ and os.environ['SINGLE_BUILD'] == 'True'):
    intersphinx_mapping = {
        'lxd': ('https://documentation.ubuntu.com/lxd/stable-5.21/', None),
        'microceph': ('https://canonical-microceph.readthedocs-hosted.com/en/v19.2.0-squid/', None),
        'microovn': ('https://canonical-microovn.readthedocs-hosted.com/en/latest/', None),
        'ceph': ('https://docs.ceph.com/en/latest/', None),
    }
elif ('READTHEDOCS' in os.environ) and (os.environ['READTHEDOCS'] == 'True'):
    intersphinx_mapping = {
        'lxd': (os.environ['PATH_PREFIX'] + 'lxd/', os.environ['READTHEDOCS_OUTPUT'] + 'html/lxd/objects.inv'),
        'microceph': (os.environ['PATH_PREFIX'] + 'microceph/', os.environ['READTHEDOCS_OUTPUT'] + 'html/microceph/objects.inv'),
        'microovn': (os.environ['PATH_PREFIX'] + 'microovn/', os.environ['READTHEDOCS_OUTPUT'] + 'html/microovn/objects.inv'),
        'ceph': ('https://docs.ceph.com/en/latest/', None)
    }
else:
    intersphinx_mapping = {
        'lxd': ('/lxd/', '_build/lxd/objects.inv'),
        'microceph': ('/microceph/', '_build/microceph/objects.inv'),
        'microovn': ('/microovn/', '_build/microovn/objects.inv'),
        'ceph': ('https://docs.ceph.com/en/latest/', None)
    }

# Define a :center: role that can be used to center the content of table cells.
rst_prolog = '''
.. role:: center
   :class: align-center
'''

if not ('SINGLE_BUILD' in os.environ and os.environ['SINGLE_BUILD'] == 'True'):
    exec(compile(source=open('.sphinx/_integration/add_config.py').read(), filename='.sphinx/_integration/add_config.py', mode='exec'))
    custom_html_static_path = ['integration/microcloud/_static']
    custom_templates_path = ['integration/microcloud/_templates']
    redirects['../index'] = 'microcloud/'
    custom_tags.append('integrated')
