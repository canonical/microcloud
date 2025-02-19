(howto-contribute)=
# How to contribute to MicroCloud

The MicroCloud team appreciates contributions to the project, through pull requests, issues on the GitHub repository, or discussions or questions on the forum.

Check the following guidelines before contributing to the project.

## Code of Conduct

When contributing, you must adhere to the Code of Conduct.
MicroCloud follows the [Ubuntu Code of Conduct](https://ubuntu.com/community/ethos/code-of-conduct).

## License and copyright

By default, any contribution to this project is made under the AGPLv3 license.
See the [license](https://github.com/canonical/microcloud/blob/main/COPYING) in the MicroCloud GitHub repository for detailed information.

All contributors must sign the [Canonical contributor license agreement](https://ubuntu.com/legal/contributors), which gives Canonical permission to use the contributions.
The author of a change remains the copyright holder of their code (no copyright assignment).

## Pull requests

Propose your changes to this project as pull requests on [GitHub](https://github.com/canonical/microcloud).

Proposed changes will then go through review there and once approved, be merged in the main branch.

### Commit structure

Use separate commits for every logical change, and for changes to different components.
Prefix your commits with the component that is affected, using the code tree structure.
For example, prefix a commit that updates the MicroCloud service with `microcloud/service:`.

This structure makes it easier for contributions to be reviewed and also greatly simplifies the process of porting fixes to other branches.

### Developer Certificate of Origin

To improve tracking of contributions to this project we use the DCO 1.1 and use a “sign-off” procedure for all changes going into the branch.

The sign-off is a simple line at the end of the explanation for the commit which certifies that you wrote it or otherwise have the right to pass it on as an open-source contribution.

```{code}
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

An example of a valid sign-off line is:

    Signed-off-by: Random J Developer <random@developer.org>

Use a known identity and a valid e-mail address, and make sure that you have signed the [Canonical contributor license agreement](https://ubuntu.com/legal/contributors).
Sorry, no anonymous contributions are allowed.

We also require each commit be individually signed-off by their author, even when part of a larger set.
You may find git {command}`commit -s` useful.

## Contribute to the documentation

We want MicroCloud to be as easy and straight-forward to use as possible.
Therefore, we aim to provide documentation that contains the information that users need to work with MicroCloud, that covers all common use cases, and that answers typical questions.

You can contribute to the documentation in various different ways.
We appreciate your contributions!

Typical ways to contribute are:

- Add or update documentation for new features or feature improvements that you contribute to the code.
  We'll review the documentation update and merge it together with your code.
- Add or update documentation that clarifies any doubts you had when working with the product.
  Such contributions can be done through a pull request or through a post in the [discussion forum](https://discourse.ubuntu.com/c/lxd/microcloud/).
  Tutorials and other documentation posted in the forum will be considered for inclusion in the docs (through a link or by including the actual content).
- To request a fix to the documentation, open a [documentation issue](https://github.com/canonical/microcloud/issues/new) on GitHub.
  We'll evaluate the issue and update the documentation accordingly.
- Post a question or a suggestion on the [discussion forum](https://discourse.ubuntu.com/c/lxd/microcloud/).
  We'll monitor the posts and, if needed, update the documentation accordingly.

### Documentation framework

MicroCloud's documentation is built with [Sphinx](https://www.sphinx-doc.org/en/master/index.html) and hosted on [Read the Docs](https://about.readthedocs.com/). <!-- wokeignore:rule=master -->

It is written in [Markdown](https://commonmark.org/) with [MyST](https://myst-parser.readthedocs.io/) extensions.
For syntax help and guidelines, see the [MyST style guide](https://canonical-documentation-with-sphinx-and-readthedocscom.readthedocs-hosted.com/style-guide-myst/) and the [documentation cheat sheet](cheat-sheet-myst) ([source](https://raw.githubusercontent.com/canonical/microcloud/main/doc/doc-cheat-sheet-myst.md)).

For structuring, the documentation uses the [Diátaxis](https://diataxis.fr/) approach.

### Build the documentation

To build the documentation, run `make doc` from the root directory of the repository.
This command installs the required tools and renders the output to the {file}`doc/_build/` directory.
To update the documentation for changed files only (without re-installing the tools), run `make doc-html`.

Before opening a pull request, make sure that the documentation builds without any warnings (warnings are treated as errors).
To preview the documentation locally, run {command}`make doc-serve` and go to [`http://localhost:8000`](http://localhost:8000) to view the rendered documentation.

When you open a pull request, a preview of the documentation output is built automatically.
To see the output, view the details for the `docs/readthedocs.com:canonical-microcloud` check on the pull request.

### Automatic documentation checks

GitHub runs automatic checks on the documentation to verify the spelling, the validity of links, and the use of inclusive language.

You can (and should!) run these tests locally as well with the following commands:

- Check the spelling: {command}`make doc-spelling`
- Check the validity of links: {command}`make doc-linkcheck`
- Check for inclusive language: {command}`make doc-woke`
