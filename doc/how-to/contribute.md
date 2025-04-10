(howto-contribute)=
# How to contribute to MicroCloud

% Include content from [../../CONTRIBUTING.md](../../CONTRIBUTING.md)
```{include} ../../CONTRIBUTING.md
    :start-after: <!-- Include start contributing -->
    :end-before: <!-- Include end contributing -->
```

## Contribute to the documentation

We strive to make MicroCloud as easy and straightforward to use as possible. To achieve this, our documentation aims to provide the information users need, cover all common use cases, and answer typical questions.

You can contribute to the documentation in several ways. We appreciate your help!

Only use this repository for contributions to the MicroCloud documentation. Submit pull requests to the integrated documentation sets at their respective standalone GitHub repositories:

- [LXD](https://github.com/canonical/lxd)
- [MicroCeph](https://github.com/canonical/microceph)
- [MicroOVN](https://github.com/canonical/microovn)

### Ways to contribute

Document new features or improvements you contribute to the code.
: - Submit documentation updates in pull requests alongside your code changes. We will review and merge them together with the code.

Clarify concepts or common questions based on your own experience.
: - Submit a pull request with your documentation improvements.

Report documentation issues by opening an issue in [GitHub](https://github.com/canonical/microcloud/issues).
: - We will evaluate and update the documentation as needed.

Ask questions or suggest improvements in the [MicroCloud forum](https://discourse.ubuntu.com/c/lxd/microcloud/145).
: - We monitor discussions and update the documentation when necessary.

If you contribute images to `doc/images`:
- Use **SVG** or **PNG** formats.
- Optimize PNG images for smaller file size using a tool like [TinyPNG](https://tinypng.com/) (web-based), [OptiPNG](https://optipng.sourceforge.net/) (CLI-based), or similar.

### Documentation framework

The MicroCloud documentation and its integrated documentation sets are built with [Sphinx](https://www.sphinx-doc.org/) and hosted on [Read the Docs](https://about.readthedocs.com/). For structuring, all use the [Diátaxis](https://diataxis.fr/) approach.

The MicroCloud and LXD documentation sets are written in [Markdown](https://commonmark.org/) with [MyST](https://myst-parser.readthedocs.io/) extensions. For syntax help and guidelines, see the [MyST style guide](https://canonical-documentation-with-sphinx-and-readthedocscom.readthedocs-hosted.com/style-guide-myst/) and the [documentation cheat sheet](cheat-sheet-myst) ([source](https://raw.githubusercontent.com/canonical/microcloud/main/doc/doc-cheat-sheet-myst.md)).

The MicroCeph and MicroOVN documentation sets are written in a documentation markup language called [reStructuredText](https://docutils.sourceforge.io/rst.html) (`.rst`). Differences in functionality are few; however, syntax differs.

### Build the documentation

To build the documentation, run `make doc` from the root directory of the repository.
This command installs the required tools and renders the output to the {file}`doc/_build/` directory.
To update the documentation for changed files only (without re-installing the tools), run `make doc-html`.

Before opening a pull request, make sure that the documentation builds without any warnings (warnings are treated as errors).
To preview the documentation locally, run {command}`make doc-serve` and go to [`http://localhost:8000`](http://localhost:8000) to view the rendered documentation.

When you open a pull request, a preview of the documentation hosted by Read the Docs is built automatically.
To see this, view the details for the `docs/readthedocs.com:canonical-microcloud` check on the pull request. Others can also use this preview to validate your changes.

### Automatic documentation checks

GitHub runs automatic checks on the documentation to verify the spelling, the validity of links, and the use of inclusive language.

You can (and should!) run these tests locally before pushing your changes:

- Check the spelling: {command}`make doc-spelling` (use {command}`make doc-spellcheck` to check without rebuilding)
- Check the validity of links: {command}`make doc-linkcheck`
- Check for inclusive language: {command}`make doc-woke`

### Links between integrated documentation sets

To link from the MicroCloud documentation to the other integrated documentation sets, use the `{ref}` role in combination with a project prefix and existing reference/link target. This allows for versions compatible with the selected MicroCloud documentation version to be shown.

#### Project prefixes

- LXD: `lxd`
- MicroCeph: `microceph`
- MicroOVN: `microovn`

Example:

```
See {ref}`lxd:security` for details.
```
