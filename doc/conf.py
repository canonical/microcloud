import datetime

# Configuration file for the Sphinx documentation builder.
#
# For the full list of built-in configuration values, see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Project information -----------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#project-information

project = 'MicroCloud'
author = 'Canonical Group Ltd'
copyright = "%s, %s" % (datetime.date.today().year, author)
release = '0.99'

# -- General configuration ---------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#general-configuration

extensions = [
    'sphinx_design',
    'sphinx_tabs.tabs',
    'sphinx_reredirects',
    'myst_parser',
    'sphinx.ext.intersphinx',
    "youtube-links",
    "related-links",
    "custom-rst-roles",
    "terminal-output"]

exclude_patterns = ['_build', 'Thumbs.db', '.DS_Store', '.sphinx']

source_suffix = {
    '.rst': 'restructuredtext',
    '.md': 'markdown',
}

intersphinx_mapping = {
    'lxd': ('https://linuxcontainers.org/lxd/docs/master/', None)
}

# -- Options for HTML output -------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#options-for-html-output

html_theme = 'furo'
html_last_updated_fmt = ""
html_permalinks_icon = "¶"
html_theme_options = {
    "light_css_variables": {
        "color-sidebar-background-border": "none",
        "font-stack": "Ubuntu, -apple-system, Segoe UI, Roboto, Oxygen, Cantarell, Fira Sans, Droid Sans, Helvetica Neue, sans-serif",
        "font-stack--monospace": "Ubuntu Mono, Consolas, Monaco, Courier, monospace",
        "color-foreground-primary": "#111",
        "color-foreground-secondary": "var(--color-foreground-primary)",
        "color-foreground-muted": "#333",
        "color-background-secondary": "#FFF",
        "color-background-hover": "#f2f2f2",
        "color-brand-primary": "#111",
        "color-brand-content": "#06C",
        "color-api-background": "#cdcdcd",
        "color-inline-code-background": "rgba(0,0,0,.03)",
        "color-sidebar-link-text": "#111",
        "color-sidebar-item-background--current": "#ebebeb",
        "color-sidebar-item-background--hover": "#f2f2f2",
        "toc-font-size": "var(--font-size--small)",
        "color-admonition-title-background--note": "var(--color-background-primary)",
        "color-admonition-title-background--tip": "var(--color-background-primary)",
        "color-admonition-title-background--important": "var(--color-background-primary)",
        "color-admonition-title-background--caution": "var(--color-background-primary)",
        "color-admonition-title--note": "#24598F",
        "color-admonition-title--tip": "#24598F",
        "color-admonition-title--important": "#C7162B",
        "color-admonition-title--caution": "#F99B11",
        "color-highlighted-background": "#EbEbEb",
        "color-link-underline": "var(--color-background-primary)",
        "color-link-underline--hover": "var(--color-background-primary)",
    },
    "dark_css_variables": {
        "color-foreground-secondary": "var(--color-foreground-primary)",
        "color-foreground-muted": "#CDCDCD",
        "color-background-secondary": "var(--color-background-primary)",
        "color-background-hover": "#666",
        "color-brand-primary": "#fff",
        "color-brand-content": "#06C",
        "color-sidebar-link-text": "#f7f7f7",
        "color-sidebar-item-background--current": "#666",
        "color-sidebar-item-background--hover": "#333",
        "color-admonition-background": "transparent",
        "color-admonition-title-background--note": "var(--color-background-primary)",
        "color-admonition-title-background--tip": "var(--color-background-primary)",
        "color-admonition-title-background--important": "var(--color-background-primary)",
        "color-admonition-title-background--caution": "var(--color-background-primary)",
        "color-admonition-title--note": "#24598F",
        "color-admonition-title--tip": "#24598F",
        "color-admonition-title--important": "#C7162B",
        "color-admonition-title--caution": "#F99B11",
        "color-highlighted-background": "#666",
        "color-link-underline": "var(--color-background-primary)",
        "color-link-underline--hover": "var(--color-background-primary)",
    },
}


html_static_path = ['.sphinx/_static']
html_css_files = [
    'custom.css'
]

# Set up redirects (https://documatt.gitlab.io/sphinx-reredirects/usage.html)
# For example: "explanation/old-name.html": "../how-to/prettify.html",
redirects = {}
