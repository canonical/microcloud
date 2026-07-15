 // Replace oldDomain with newDomain
 const microcloud_oldDomain = 'canonical-microcloud.readthedocs-hosted.com';
 const microcloud_newDomain = 'canonical.com/microcloud/docs';

 function microcloud_escapeRegExp(value) {
     return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
 }

 function microcloud_overwriteMatchingAnchorUrls(container) {
     if (!container) return;

     const anchors = container.querySelectorAll('a[href], link[href]');
     const oldDomainRegex = new RegExp(microcloud_escapeRegExp(microcloud_oldDomain), 'g');

     anchors.forEach(anchor => {
         anchor.href = anchor.href.replace(oldDomainRegex, microcloud_newDomain);
     });
 }

 microcloud_overwriteMatchingAnchorUrls(document.querySelector('header'));

 // Use a MutationObserver to wait for the RTD flyout element to appear in the DOM
 const microcloud_observer = new MutationObserver(function(mutations, obs) {

     const rtdFlyout = document.querySelector('readthedocs-flyout');
     if (!rtdFlyout) return;

     obs.disconnect();

     rtdFlyout.addEventListener('click', function() {
         const shadowRoot = rtdFlyout.shadowRoot;
         if (!shadowRoot) return;

         microcloud_overwriteMatchingAnchorUrls(shadowRoot);
     });
 });

 microcloud_observer.observe(document.body, { childList: true, subtree: true });
