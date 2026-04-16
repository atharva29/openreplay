var notifications=(function(){"use strict";function d(e){return e==null||typeof e=="function"?{main:e}:e}const l=d(()=>{async function e(n){const t=document.createElement("textarea");t.value=n,t.setAttribute("readonly",""),t.style.position="absolute",t.style.left="-9999px",document.body.appendChild(t),t.select(),document.execCommand("copy"),document.body.removeChild(t)}function o(){const n=`
    .or-flex{display:flex}
    .or-items-center {align-items:center}
    .or-gap-3 {gap: .25rem}
    .or-spinner {
        width: 18px;
        height: 18px;
        border: 2px solid rgba(0, 0, 0, 0.1);
        border-radius: 50%;
        border-top-color: #394dfe;
        animation: spin 0.6s linear infinite;
      }

      @keyframes spin {
        to {
          transform: rotate(360deg);
        }
      }
    `,t=document.createElement("style");t.textContent=n,document.head.appendChild(t)}function c(n){const a=`
    <div class="or-flex or-gap-3 or-items-center">
      <div class="or-spinner"></div>
      <span>${n.data.message||"Recording has started successfully."}</span>
     </div>
    `,i=document.createElement("div"),p={position:"fixed",bottom:"2rem",right:"2rem",backgroundColor:"#E2E4F6",color:"black",padding:"1.5rem",borderRadius:"0.75rem",opacity:"0.9",transition:"opacity 300ms",zIndex:99999999};Object.assign(i.style,p),i.innerHTML=a,document.body.appendChild(i),i.offsetHeight,setTimeout(()=>{i.style.opacity="0",setTimeout(()=>{document.body.removeChild(i)},300)},4500)}function u(){function n(t){t.data.type==="ornotif:display"&&c(t),t.data.type==="ornotif:copy"&&e(t.data.url).then(()=>{c({data:{message:"Link copied to clipboard and new tab opened"}})}).catch(a=>{console.error(a)}),t.data.type==="ornotif:stop"&&window.removeEventListener("message",n)}return window.addEventListener("message",n),function(){window.removeEventListener("message",n)}}o(),window.__or_clear_notifications||(window.__or_clear_notifications=u())});function f(){}function r(e,...o){}const s={debug:(...e)=>r(console.debug,...e),log:(...e)=>r(console.log,...e),warn:(...e)=>r(console.warn,...e),error:(...e)=>r(console.error,...e)};return(()=>{try{}catch(o){throw s.error('Failed to initialize plugins for "notifications"',o),o}let e;try{e=l.main(),e instanceof Promise&&(e=e.catch(o=>{throw s.error('The unlisted script "notifications" crashed on startup!',o),o}))}catch(o){throw s.error('The unlisted script "notifications" crashed on startup!',o),o}return e})()})();
notifications;