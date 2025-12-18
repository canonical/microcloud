# Links to other doc sets (used in the header)
# All paths are relative to the URL of one doc set
html_context['microcloud_path'] = "../microcloud"
html_context['microcloud_tag'] = "../microcloud/_static/tag.png"
html_context['lxd_path'] = "../lxd"
html_context['lxd_tag'] = "../lxd/_static/tag.png"
html_context['microceph_path'] = "../microceph"
html_context['microceph_tag'] = "../microceph/_static/tag.png"
html_context['microovn_path'] = "../microovn"
html_context['microovn_tag'] = "../microovn/_static/microovn.png"

if project == "LXD":
    html_baseurl = "https://documentation.ubuntu.com/lxd/stable-5.21/"
    html_js_files.append('rtd-search.js')
    tags.add('integrated')
elif project == "MicroCeph":
    html_baseurl = "https://canonical-microceph.readthedocs-hosted.com/en/latest/"
    # Override default header templates
    templates_path = globals().get('templates_path', []) + [".sphinx/_templates"]
    # Override default header styles
    html_static_path = globals().get('html_static_path', []) + [".sphinx/_static"]
    html_css_files = globals().get('html_css_files', []) + ['override-header.css']
    # Add rtd-search.js to the list of JS files
    html_js_files = globals().get('html_js_files', []) + ['rtd-search.js']
    # Add "integrated" to the list of custom tags
    custom_tags = globals().get('custom_tags', []) + ['integrated']
elif project == "MicroOVN":
    html_baseurl = "https://canonical-microovn.readthedocs-hosted.com/en/24.03/"
    custom_html_js_files.append('rtd-search.js')
    custom_tags.append('integrated')

