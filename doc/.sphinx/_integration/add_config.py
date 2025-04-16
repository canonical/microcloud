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

# Add the search JavaScript file
custom_html_js_files.append('rtd-search.js')

if project == "LXD":
    html_baseurl = "https://documentation.ubuntu.com/lxd/en/latest/"
elif project == "MicroCeph":
    html_baseurl = "https://canonical-microceph.readthedocs-hosted.com/en/latest/"
elif project == "MicroOVN":
    html_baseurl = "https://canonical-microovn.readthedocs-hosted.com/en/latest/"

# Include integrated tag
custom_tags.append('integrated')
