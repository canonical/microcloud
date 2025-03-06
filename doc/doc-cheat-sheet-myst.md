---
orphan: true
myst:
    substitutions:
      reuse_key: "This is **included** text."
      advanced_reuse_key: "This is a substitution that includes a code block:
                         ```
                         code block
                         ```"
---

<!-- vale off -->

(cheat-sheet-myst)=
# Markdown/MyST cheat sheet

<!-- vale on -->

This file contains the syntax for commonly used Markdown and MyST markup.
Open it in your text editor to quickly copy and paste the markup you need.

See the [MyST style guide](https://canonical-documentation-with-sphinx-and-readthedocscom.readthedocs-hosted.com/style-guide-myst/) for detailed information and conventions.

Also see the [MyST documentation](https://myst-parser.readthedocs.io/en/latest/index.html) for detailed information on MyST, and the [Canonical Documentation Style Guide](https://docs.ubuntu.com/styleguide/en) for general style conventions.

## H2 heading

### H3 heading

#### H4 heading

##### H5 heading

## Inline formatting

- {guilabel}`UI element`
- `code`
- {command}`command`
- {kbd}`Key`
- *Italic*
- **Bold**

## Code blocks

Start a code block:

    code:
      - example: true

```
# Demonstrate a code block
code:
  - example: true
```

```yaml
# Demonstrate a code block
code:
  - example: true
```

(a_section_target_myst)=
## Links

- [Canonical website](https://canonical.com/)
- {ref}`a_section_target_myst`
- {ref}`Link text <a_section_target_myst>`
- {doc}`index`
- {doc}`Link text <index>`

## Navigation

Use the following syntax::

    ```{toctree}
    :hidden:

    sub-page1
    sub-page2
    ```

## Lists

1. Step 1
   - Item 1
      * Sub-item
   - Item 2
     1. Sub-step 1
     1. Sub-step 2
1. Step 2
   1. Sub-step 1
      - Item
   1. Sub-step 2

Term 1
: Definition

Term 2
: Definition

## Tables

## Markdown tables

| Header 1                           | Header 2 |
|------------------------------------|----------|
| Cell 1<br>Second paragraph         | Cell 2   |
| Cell 3                             | Cell 4   |

Centered:

| Header 1                           | Header 2 |
|:----------------------------------:|:--------:|
| Cell 1<br>Second paragraph         | Cell 2   |
| Cell 3                             | Cell 4   |

## List tables

```{list-table}
   :header-rows: 1

* - Header 1
  - Header 2
* - Cell 1

    Second paragraph
  - Cell 2
* - Cell 3
  - Cell 4
```

Centered:

```{list-table}
   :header-rows: 1
   :align: center

* - Header 1
  - Header 2
* - Cell 1

    Second paragraph
  - Cell 2
* - Cell 3
  - Cell 4
```

## Notes

```{note}
A note.
```

```{tip}
A tip.
```

```{important}
Important information
```

```{caution}
This might damage your hardware!
```

## Images

![Alt text](https://assets.ubuntu.com/v1/b3b72cb2-canonical-logo-166.png)

```{figure} https://assets.ubuntu.com/v1/b3b72cb2-canonical-logo-166.png
   :width: 100px
   :alt: Alt text

   Figure caption
```

## Reuse

### Keys

Keys can be defined at the top of a file, or in a `myst_substitutions` option in `conf.py`.

{{reuse_key}}

{{advanced_reuse_key}}

### File inclusion

## Tabs

````{tabs}
```{group-tab} Tab 1

Content Tab 1
```

```{group-tab} Tab 2
Content Tab 2
```
````

## Glossary

```{glossary}

some term
  Definition of the example term.
```

{term}`some term`

## More useful markup

- ```{versionadded} X.Y
- {abbr}`API (Application Programming Interface)`

----

## Custom extensions

Related links at the top of the page (surrounded by `---`):

    relatedlinks: https://github.com/canonical/lxd-sphinx-extensions, [RTFM](https://www.google.com)
    discourse: 12345

Terms that should not be checked by the spelling checker: {spellexception}`PurposelyWrong`

A single-line terminal view that separates input from output:

```{terminal}
   :input: command
   :user: root
   :host: vampyr
   :dir: /home/user/directory/

the output
```

A multi-line version of the same:

```{terminal}
   :user: root
   :host: vampyr
   :dir: /home/user/directory/

:input: command 1
output 1
:input: command 2
output 2
```

A link to a YouTube video:

```{youtube} https://www.youtube.com/watch?v=iMLiK1fX4I0
   :title: Demo
```
