import{aS as ve,aR as be,r as h,aT as Fe,dx as Re,dy as Ne,j as e,aN as Z,aV as ye,aW as je,aM as We,cG as De,aP as Ae,dz as Ge,bK as $e,bf as se,bV as Ve,bg as Ce,b5 as Je,b7 as Ye,bT as Ze,u as oe,aC as qe,B as g,T as d,c9 as ae,a7 as B,t as m,a1 as M,A as F,P as z,F as he,dA as Xe,dv as ge,aJ as fe,p as Me,q as Ee,s as J,cq as Qe,C as et,dB as me,e as K,V as tt,G as ze,av as st,d as we,I as ke,W as at,a9 as rt,X as ot,Y as it,Z as nt,O as le,$ as lt,am as pt,cm as O,cp as dt,ac as Y,af as Ke,x as ct,aD as ut,aE as ht,aF as N,bs as gt,aG as ft,aA as _e}from"./index-Bee-2m60.js";import{S as E}from"./Stack-jw0dU-im.js";import{S as re}from"./Switch-Do74-Im_.js";import{T as mt}from"./TwoFactorSettings-ufTbAt_7.js";import{T as Se}from"./TableContainer-D1iWEPgB.js";import{T as Te,a as Ie,b as G,c as b,d as Ue}from"./TableRow-D-06YzrA.js";import{C as ee}from"./Chip-D6zH9sZH.js";import{A as pe,a as de,b as ce}from"./AccordionSummary-RMO42XGK.js";import{A as xt}from"./Autocomplete--5OsRn0P.js";import vt from"./ApiKeys-BWwFzKZv.js";import"./apiKeyApi-BYMHTHvF.js";import"./TablePagination-BgOOEatw.js";import"./LastPage-CxkhVsqo.js";import"./Toolbar-OJtXZrr5.js";function bt(t){return ve("MuiFormGroup",t)}be("MuiFormGroup",["root","row","error"]);const yt=t=>{const{classes:n,row:s,error:c}=t;return je({root:["root",s&&"row",c&&"error"]},bt,n)},jt=Z("div",{name:"MuiFormGroup",slot:"Root",overridesResolver:(t,n)=>{const{ownerState:s}=t;return[n.root,s.row&&n.row]}})({display:"flex",flexDirection:"column",flexWrap:"wrap",variants:[{props:{row:!0},style:{flexDirection:"row"}}]}),xe=h.forwardRef(function(n,s){const c=Fe({props:n,name:"MuiFormGroup"}),{className:p,row:x=!1,..._}=c,I=Re(),k=Ne({props:c,muiFormControl:I,states:["error"]}),f={...c,row:x,error:k.error},y=yt(f);return e.jsx(jt,{className:ye(y.root,p),ownerState:f,ref:s,..._})}),Ct=We(e.jsx("path",{d:"M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8z"})),wt=We(e.jsx("path",{d:"M8.465 8.465C9.37 7.56 10.62 7 12 7C14.76 7 17 9.24 17 12C17 13.38 16.44 14.63 15.535 15.535C14.63 16.44 13.38 17 12 17C9.24 17 7 14.76 7 12C7 10.62 7.56 9.37 8.465 8.465Z"})),kt=Z("span",{name:"MuiRadioButtonIcon",shouldForwardProp:De})({position:"relative",display:"flex"}),_t=Z(Ct,{name:"MuiRadioButtonIcon"})({transform:"scale(1)"}),St=Z(wt,{name:"MuiRadioButtonIcon"})(Ae(({theme:t})=>({left:0,position:"absolute",transform:"scale(0)",transition:t.transitions.create("transform",{easing:t.transitions.easing.easeIn,duration:t.transitions.duration.shortest}),variants:[{props:{checked:!0},style:{transform:"scale(1)",transition:t.transitions.create("transform",{easing:t.transitions.easing.easeOut,duration:t.transitions.duration.shortest})}}]})));function Be(t){const{checked:n=!1,classes:s={},fontSize:c}=t,p={...t,checked:n};return e.jsxs(kt,{className:s.root,ownerState:p,children:[e.jsx(_t,{fontSize:c,className:s.background,ownerState:p}),e.jsx(St,{fontSize:c,className:s.dot,ownerState:p})]})}const He=h.createContext(void 0);function Tt(){return h.useContext(He)}function It(t){return ve("MuiRadio",t)}const Pe=be("MuiRadio",["root","checked","disabled","colorPrimary","colorSecondary","sizeSmall"]),Ut=t=>{const{classes:n,color:s,size:c}=t,p={root:["root",`color${se(s)}`,c!=="medium"&&`size${se(c)}`]};return{...n,...je(p,It,n)}},Pt=Z(Ve,{shouldForwardProp:t=>De(t)||t==="classes",name:"MuiRadio",slot:"Root",overridesResolver:(t,n)=>{const{ownerState:s}=t;return[n.root,s.size!=="medium"&&n[`size${se(s.size)}`],n[`color${se(s.color)}`]]}})(Ae(({theme:t})=>({color:(t.vars||t).palette.text.secondary,[`&.${Pe.disabled}`]:{color:(t.vars||t).palette.action.disabled},variants:[{props:{color:"default",disabled:!1,disableRipple:!1},style:{"&:hover":{backgroundColor:t.alpha((t.vars||t).palette.action.active,(t.vars||t).palette.action.hoverOpacity)}}},...Object.entries(t.palette).filter(Ce()).map(([n])=>({props:{color:n,disabled:!1,disableRipple:!1},style:{"&:hover":{backgroundColor:t.alpha((t.vars||t).palette[n].main,(t.vars||t).palette.action.hoverOpacity)}}})),...Object.entries(t.palette).filter(Ce()).map(([n])=>({props:{color:n,disabled:!1},style:{[`&.${Pe.checked}`]:{color:(t.vars||t).palette[n].main}}})),{props:{disableRipple:!1},style:{"&:hover":{"@media (hover: none)":{backgroundColor:"transparent"}}}}]})));function Lt(t,n){return typeof n=="object"&&n!==null?t===n:String(t)===String(n)}const Ft=e.jsx(Be,{checked:!0}),Rt=e.jsx(Be,{}),ue=h.forwardRef(function(n,s){const c=Fe({props:n,name:"MuiRadio"}),{checked:p,checkedIcon:x=Ft,color:_="primary",icon:I=Rt,name:k,onChange:f,size:y="medium",className:r,disabled:l,disableRipple:U=!1,slots:L={},slotProps:R={},inputProps:v,...j}=c,C=Re();let o=l;C&&typeof o>"u"&&(o=C.disabled),o??=!1;const i={...c,disabled:o,disableRipple:U,color:_,size:y},w=Ut(i),P=Tt();let W=p;const A=Ge(f,P&&P.onChange);let D=k;P&&(typeof W>"u"&&(W=Lt(P.value,c.value)),typeof D>"u"&&(D=P.name));const H=R.input??v,[ie,ne]=$e("root",{ref:s,elementType:Pt,className:ye(w.root,r),shouldForwardComponentProp:!0,externalForwardedProps:{slots:L,slotProps:R,...j},getSlotProps:X=>({...X,onChange:(Q,...q)=>{X.onChange?.(Q,...q),A(Q,...q)}}),ownerState:i,additionalProps:{type:"radio",icon:h.cloneElement(I,{fontSize:I.props.fontSize??y}),checkedIcon:h.cloneElement(x,{fontSize:x.props.fontSize??y}),disabled:o,name:D,checked:W,slots:L,slotProps:{input:typeof H=="function"?H(i):H}}});return e.jsx(ie,{...ne,classes:w})});function Wt(t){return ve("MuiRadioGroup",t)}be("MuiRadioGroup",["root","row","error"]);const Dt=t=>{const{classes:n,row:s,error:c}=t;return je({root:["root",s&&"row",c&&"error"]},Wt,n)},At=h.forwardRef(function(n,s){const{actions:c,children:p,className:x,defaultValue:_,name:I,onChange:k,value:f,...y}=n,r=h.useRef(null),l=Dt(n),[U,L]=Je({controlled:f,default:_,name:"RadioGroup"});h.useImperativeHandle(c,()=>({focus:()=>{let C=r.current.querySelector("input:not(:disabled):checked");C||(C=r.current.querySelector("input:not(:disabled)")),C&&C.focus()}}),[]);const R=Ye(s,r),v=Ze(I),j=h.useMemo(()=>({name:v,onChange(C){L(C.target.value),k&&k(C,C.target.value)},value:U}),[v,k,L,U]);return e.jsx(He.Provider,{value:j,children:e.jsx(xe,{role:"radiogroup",ref:R,className:ye(l.root,x),...y,children:p})})});function qt({config:t,onChange:n}){const{t:s}=oe(),{refreshSettings:c}=qe(),p=x=>{n({...t,showWatermark:x}),localStorage.setItem("showWatermark",String(x)),setTimeout(()=>c(),100)};return e.jsx(E,{spacing:4,children:e.jsxs(g,{children:[e.jsxs(d,{variant:"h6",gutterBottom:!0,sx:{display:"flex",alignItems:"center"},children:[e.jsx(ae,{sx:{mr:1}})," ",s("settings.basicConfig")]}),e.jsx(B,{sx:{mb:3}}),e.jsxs(E,{spacing:3,children:[e.jsx(m,{label:s("settings.siteName"),value:t.siteName||"",onChange:x=>n({...t,siteName:x.target.value}),helperText:s("settings.displayOnTitle"),fullWidth:!0}),e.jsx(M,{control:e.jsx(re,{checked:t.showWatermark??!1,onChange:x=>p(x.target.checked)}),label:e.jsxs(g,{children:[e.jsx(d,{variant:"body1",children:s("settings.showWatermark")}),e.jsx(d,{variant:"caption",color:"text.secondary",children:s("settings.showWatermarkHelper")})]})})]})]})})}const Le=["oidc","feishu","lark","dingtalk","wecom"];function Mt({config:t,onChange:n}){const{t:s}=oe(),c=r=>{switch(r){case"oidc":return s("settings.ssoProviderOIDC");case"feishu":return s("settings.ssoProviderFeishu");case"lark":return s("settings.ssoProviderLark");case"dingtalk":return s("settings.ssoProviderDingTalk");case"wecom":return s("settings.ssoProviderWeCom");default:return r}},p=()=>_(t.sso.provider),x=r=>_(r),_=r=>{const l={feishu:{title:"📋 Feishu Configuration Example:",provider:"• Provider: Feishu",authUrl:"• Authorization URL: https://open.feishu.cn/open-apis/authen/v1/authorize",tokenUrl:"• Token URL: https://open.feishu.cn/open-apis/authen/v1/oidc/access_token",userInfoUrl:"• User Info URL: https://open.feishu.cn/open-apis/authen/v1/user_info",note:s("settings.ssoExampleNoteFeishu"),authPlaceholder:"https://open.feishu.cn/open-apis/authen/v1/authorize",tokenPlaceholder:"https://open.feishu.cn/open-apis/authen/v1/oidc/access_token",userInfoPlaceholder:"https://open.feishu.cn/open-apis/authen/v1/user_info"},lark:{title:"📋 Lark Configuration Example:",provider:"• Provider: Lark",authUrl:"• Authorization URL: https://open.larksuite.com/open-apis/authen/v1/authorize",tokenUrl:"• Token URL: https://open.larksuite.com/open-apis/authen/v1/oidc/access_token",userInfoUrl:"• User Info URL: https://open.larksuite.com/open-apis/authen/v1/user_info",note:s("settings.ssoExampleNoteLark"),authPlaceholder:"https://open.larksuite.com/open-apis/authen/v1/authorize",tokenPlaceholder:"https://open.larksuite.com/open-apis/authen/v1/oidc/access_token",userInfoPlaceholder:"https://open.larksuite.com/open-apis/authen/v1/user_info"},dingtalk:{title:"📋 DingTalk Configuration Example:",provider:"• Provider: DingTalk",authUrl:"• Authorization URL: https://login.dingtalk.com/oauth2/auth",tokenUrl:"• Token URL: https://api.dingtalk.com/v1.0/oauth2/accessToken",userInfoUrl:"• User Info URL: https://api.dingtalk.com/v1.0/contacts/users/me",note:s("settings.ssoExampleNoteDingtalk"),authPlaceholder:"https://login.dingtalk.com/oauth2/auth",tokenPlaceholder:"https://api.dingtalk.com/v1.0/oauth2/accessToken",userInfoPlaceholder:"https://api.dingtalk.com/v1.0/contacts/users/me"},wecom:{title:"📋 WeCom (WeChat Work) Configuration Example:",provider:"• Provider: WeCom",authUrl:"• Authorization URL: https://open.weixin.qq.com/connect/oauth2/authorize",tokenUrl:"• Token URL: https://qyapi.weixin.qq.com/cgi-bin/gettoken",userInfoUrl:"• User Info URL: https://qyapi.weixin.qq.com/cgi-bin/user/getuserinfo",note:s("settings.ssoExampleNoteWecom"),authPlaceholder:"https://open.weixin.qq.com/connect/oauth2/authorize",tokenPlaceholder:"https://qyapi.weixin.qq.com/cgi-bin/gettoken",userInfoPlaceholder:"https://qyapi.weixin.qq.com/cgi-bin/user/getuserinfo"},oidc:{title:"📋 OAuth2 / OIDC Configuration Example:",provider:"• Provider: OIDC",authUrl:"• Authorization URL: https://your-idp.com/auth",tokenUrl:"• Token URL: https://your-idp.com/token",userInfoUrl:"• User Info URL: https://your-idp.com/userinfo",note:s("settings.ssoExampleNoteOidc"),authPlaceholder:"https://your-idp.com/auth",tokenPlaceholder:"https://your-idp.com/token",userInfoPlaceholder:"https://your-idp.com/userinfo"}};return l[r]||l.feishu},I=r=>{n({...t,authMethod:r,passwordLogin:{...t.passwordLogin,enabled:r==="password"},sso:{...t.sso,enabled:r==="sso"},ldap:{...t.ldap,enabled:r==="ldap"}})},k=(r,l)=>{n({...t,passwordLogin:{...t.passwordLogin,[r]:l}})},f=(r,l)=>{n({...t,sso:{...t.sso,[r]:l}})},y=(r,l)=>{n({...t,ldap:{...t.ldap,[r]:l}})};return e.jsxs(g,{children:[e.jsx(d,{variant:"h6",gutterBottom:!0,fontWeight:"600",children:s("settings.authConfig")}),e.jsx(d,{variant:"body2",color:"text.secondary",paragraph:!0,children:s("settings.authConfigDesc")}),e.jsx(F,{severity:"warning",sx:{mb:3},children:s("settings.authWarning")}),e.jsx(z,{sx:{p:3,mb:3},children:e.jsxs(he,{component:"fieldset",children:[e.jsx(Xe,{component:"legend",sx:{fontWeight:600,color:"text.primary",mb:2},children:s("settings.selectAuthMethod")}),e.jsxs(At,{value:t.authMethod,onChange:r=>I(r.target.value),children:[e.jsx(M,{value:"password",control:e.jsx(ue,{}),label:e.jsxs(g,{sx:{display:"flex",alignItems:"center",gap:1},children:[e.jsx(ge,{fontSize:"small"}),e.jsx(d,{children:s("settings.passwordAuthTitle")})]})}),e.jsx(M,{value:"sso",control:e.jsx(ue,{}),label:e.jsxs(g,{sx:{display:"flex",alignItems:"center",gap:1},children:[e.jsx(fe,{fontSize:"small"}),e.jsx(d,{children:s("settings.ssoAuthTitle")})]})}),e.jsx(M,{value:"ldap",control:e.jsx(ue,{}),label:e.jsxs(g,{sx:{display:"flex",alignItems:"center",gap:1},children:[e.jsx(ae,{fontSize:"small"}),e.jsx(d,{children:s("settings.ldapAuthTitle")})]})})]})]})}),t.authMethod==="password"&&e.jsxs(z,{sx:{p:3,mb:3},children:[e.jsxs(g,{sx:{display:"flex",alignItems:"center",gap:1,mb:2},children:[e.jsx(ge,{color:"primary"}),e.jsx(d,{variant:"h6",fontWeight:"600",children:s("settings.passwordAuthConfig")})]}),e.jsx(B,{sx:{mb:3}}),e.jsxs(g,{sx:{display:"flex",flexDirection:"column",gap:3},children:[e.jsx(m,{fullWidth:!0,label:s("settings.minPasswordLengthLabel"),type:"number",value:t.passwordLogin.passwordMinLength,onChange:r=>k("passwordMinLength",parseInt(r.target.value)),helperText:s("settings.minPasswordLengthHelper")}),e.jsx(m,{fullWidth:!0,label:s("settings.sessionTimeoutLabel"),type:"number",value:t.passwordLogin.sessionTimeout,onChange:r=>k("sessionTimeout",parseInt(r.target.value)),helperText:s("settings.sessionTimeoutHelperText")}),e.jsx(xe,{children:e.jsx(M,{control:e.jsx(re,{checked:t.passwordLogin.passwordComplexity,onChange:r=>k("passwordComplexity",r.target.checked)}),label:s("settings.passwordComplexityLabel")})})]})]}),t.authMethod==="sso"&&e.jsxs(z,{sx:{p:3,mb:3},children:[e.jsxs(g,{sx:{display:"flex",alignItems:"center",gap:1,mb:2},children:[e.jsx(fe,{color:"primary"}),e.jsx(d,{variant:"h6",fontWeight:"600",children:s("settings.ssoAuthConfig")})]}),e.jsx(B,{sx:{mb:3}}),e.jsxs(F,{severity:"info",sx:{mb:3},children:[e.jsx(d,{variant:"body2",sx:{mb:1},children:e.jsx("strong",{children:p().title})}),e.jsxs(d,{variant:"body2",component:"div",sx:{fontFamily:"monospace",fontSize:"0.85rem",lineHeight:1.6},children:[p().provider,e.jsx("br",{}),p().authUrl,e.jsx("br",{}),p().tokenUrl,e.jsx("br",{}),p().userInfoUrl,e.jsx("br",{}),s("settings.ssoExampleScopes")]}),e.jsx(d,{variant:"body2",sx:{mt:1,color:"text.secondary"},children:p().note})]}),e.jsxs(g,{sx:{display:"flex",flexDirection:"column",gap:3},children:[e.jsxs(he,{fullWidth:!0,children:[e.jsx(Me,{id:"sso-provider-select-label",children:s("settings.providerLabel")}),e.jsx(Ee,{labelId:"sso-provider-select-label",label:s("settings.providerLabel"),value:t.sso.provider&&Le.includes(t.sso.provider)?t.sso.provider:"oidc",onChange:r=>{const l=r.target.value,U=x(l);n({...t,sso:{...t.sso,provider:l,clientId:"",clientSecret:"",authUrl:U.authPlaceholder,tokenUrl:U.tokenPlaceholder,userInfoUrl:U.userInfoPlaceholder,scopes:""}})},children:Le.map(r=>e.jsx(J,{value:r,children:c(r)},r))}),e.jsx(Qe,{children:s("settings.providerHelper")})]}),e.jsx(m,{fullWidth:!0,label:s("settings.clientIdLabel"),value:t.sso.clientId,onChange:r=>f("clientId",r.target.value),helperText:s("settings.clientIdHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.clientSecretLabel"),type:"password",value:t.sso.clientSecret,onChange:r=>f("clientSecret",r.target.value),helperText:s("settings.clientSecretHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.authUrlLabel"),value:t.sso.authUrl,onChange:r=>f("authUrl",r.target.value),placeholder:p().authPlaceholder,helperText:s("settings.authUrlHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.tokenUrlLabel"),value:t.sso.tokenUrl,onChange:r=>f("tokenUrl",r.target.value),placeholder:p().tokenPlaceholder,helperText:s("settings.tokenUrlHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.userInfoUrlLabel"),value:t.sso.userInfoUrl,onChange:r=>f("userInfoUrl",r.target.value),placeholder:p().userInfoPlaceholder,helperText:s("settings.userInfoUrlHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.redirectUrlLabel"),value:t.sso.redirectUrl,onChange:r=>f("redirectUrl",r.target.value),placeholder:s("settings.redirectUrlPlaceholder"),helperText:s("settings.redirectUrlHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.scopesLabel"),value:t.sso.scopes,onChange:r=>f("scopes",r.target.value),placeholder:s("settings.scopesPlaceholder"),helperText:s("settings.scopesHelper2")})]})]}),t.authMethod==="ldap"&&e.jsxs(z,{sx:{p:3,mb:3},children:[e.jsxs(g,{sx:{display:"flex",alignItems:"center",gap:1,mb:2},children:[e.jsx(ae,{color:"primary"}),e.jsx(d,{variant:"h6",fontWeight:"600",children:s("settings.ldapAuthConfig")})]}),e.jsx(B,{sx:{mb:3}}),e.jsxs(g,{sx:{display:"flex",flexDirection:"column",gap:3},children:[e.jsx(m,{fullWidth:!0,label:s("settings.ldapServerLabel"),value:t.ldap.server,onChange:r=>y("server",r.target.value),placeholder:s("settings.ldapServerPlaceholder"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.portLabel"),type:"number",value:t.ldap.port,onChange:r=>y("port",parseInt(r.target.value)),helperText:s("settings.portHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.bindDnLabel"),value:t.ldap.bindDn,onChange:r=>y("bindDn",r.target.value),placeholder:s("settings.bindDnPlaceholder"),helperText:s("settings.bindDnHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.bindPasswordLabel"),type:"password",value:t.ldap.bindPassword,onChange:r=>y("bindPassword",r.target.value),helperText:s("settings.bindPasswordHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.baseDnLabel"),value:t.ldap.baseDn,onChange:r=>y("baseDn",r.target.value),placeholder:s("settings.baseDnPlaceholder"),helperText:s("settings.baseDnHelper"),required:!0}),e.jsx(m,{fullWidth:!0,label:s("settings.userFilterLabel"),value:t.ldap.userFilter,onChange:r=>y("userFilter",r.target.value),placeholder:s("settings.userFilterPlaceholder"),helperText:s("settings.userFilterHelper2"),required:!0}),e.jsx(xe,{children:e.jsx(M,{control:e.jsx(re,{checked:t.ldap.useTLS,onChange:r=>y("useTLS",r.target.checked)}),label:s("settings.enableTlsLabel")})})]})]}),e.jsx(F,{severity:"info",children:e.jsxs(d,{variant:"body2",children:[e.jsx("strong",{children:s("settings.currentAuthStatus")}),t.authMethod==="password"&&s("settings.passwordAuthStatus"),t.authMethod==="sso"&&s("settings.ssoAuthStatus"),t.authMethod==="ldap"&&s("settings.ldapAuthStatus")]})})]})}const Et=[{groupKey:"system",rows:[{keyword:"auto_id",rowKey:"autoId"},{keyword:"auto_url",rowKey:"autoUrl"}]},{groupKey:"list",rows:[{keyword:"list.site",rowKey:"listSite"},{keyword:"list.type_label",rowKey:"listTypeLabel"},{keyword:"list.order",rowKey:"listOrder"},{keyword:"list.owner",rowKey:"listOwner"}]},{groupKey:"details",rows:[{keyword:"details",rowKey:"details"}]},{groupKey:"detailFields",rows:[{keyword:"detail.app_name",rowKey:"detailAppName"},{keyword:"detail.tag",rowKey:"detailTag"},{keyword:"detail.sub_type",rowKey:"detailSubType"}]},{groupKey:"misc",rows:[{keyword:"fixed:<value>",rowKey:"fixed"}]}];function $(t){const n=(t||"feishu").trim().toLowerCase();return`${typeof window<"u"?window.location.origin.replace(/\/+$/,""):""}/api/approvals/callback/release/${n}`}function zt(t,n){if(!t?.trim())return[];try{const s=JSON.parse(t);if(Array.isArray(s))return s.map(c=>{const p=String(c).trim();return p.includes("@")?p:n.find(_=>_.id===p||_.username===p||_.email===p)?.email||p}).filter(c=>c.length>0)}catch{}return t.split(/[,;\s]+/).map(s=>s.trim()).filter(s=>s.length>0)}const te={feishu:`[
  {
    "id": "release_title",
    "name": "发布标题",
    "type": "input",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "application_name",
    "name": "应用名称",
    "type": "input",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "environment",
    "name": "发布环境",
    "type": "input",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "version",
    "name": "发布版本",
    "type": "input",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "release_type",
    "name": "发布类型",
    "type": "radioV2",
    "required": true,
    "visible": true,
    "printable": true,
    "option": [
      {"value": "publish", "text": "发布"},
      {"value": "rollback", "text": "回滚"}
    ]
  },
  {
    "id": "site",
    "name": "发布站点",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true
  },
  {
    "id": "cluster",
    "name": "集群",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true
  },
  {
    "id": "namespace",
    "name": "命名空间",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true
  },
  {
    "id": "reason",
    "name": "发布说明",
    "type": "textarea",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "release_run_id",
    "name": "发布单ID",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true
  }
]`,dingtalk:`[
  {
    "name": "release_title",
    "label": "发布标题",
    "field": "title"
  },
  {
    "name": "application_name",
    "label": "应用名称",
    "field": "application_name"
  },
  {
    "name": "environment",
    "label": "发布环境",
    "field": "environment"
  },
  {
    "name": "version",
    "label": "发布版本",
    "field": "version"
  },
  {
    "name": "release_type",
    "label": "发布类型",
    "field": "release_type",
    "options": [
      {"value": "publish", "label": "发布"},
      {"value": "rollback", "label": "回滚"}
    ]
  },
  {
    "name": "reason",
    "label": "发布说明",
    "field": "reason"
  },
  {
    "name": "site",
    "label": "发布站点",
    "field": "site"
  },
  {
    "name": "cluster",
    "label": "集群",
    "field": "cluster"
  },
  {
    "name": "namespace",
    "label": "命名空间",
    "field": "namespace"
  }
]`,wechat:`[
  {
    "control": "Textarea",
    "id": "Text-release-title",
    "label": "发布标题",
    "field": "title"
  },
  {
    "control": "Text",
    "id": "Text-application-name",
    "label": "应用名称",
    "field": "application_name"
  },
  {
    "control": "Text",
    "id": "Text-environment",
    "label": "发布环境",
    "field": "environment"
  },
  {
    "control": "Text",
    "id": "Text-version",
    "label": "发布版本",
    "field": "version"
  },
  {
    "control": "Select",
    "id": "Select-release-type",
    "label": "发布类型",
    "field": "release_type",
    "options": [
      {"value": "publish", "label": "发布"},
      {"value": "rollback", "label": "回滚"}
    ]
  },
  {
    "control": "Textarea",
    "id": "Textarea-release-reason",
    "label": "发布说明",
    "field": "reason"
  },
  {
    "control": "Text",
    "id": "Text-site",
    "label": "发布站点",
    "field": "site"
  },
  {
    "control": "Text",
    "id": "Text-cluster",
    "label": "集群",
    "field": "cluster"
  },
  {
    "control": "Text",
    "id": "Text-namespace",
    "label": "命名空间",
    "field": "namespace"
  }
]`,lark:`[
  {
    "id": "release_title",
    "name": "发布标题",
    "type": "input",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "application_name",
    "name": "应用名称",
    "type": "input",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "environment",
    "name": "发布环境",
    "type": "input",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "version",
    "name": "发布版本",
    "type": "input",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "release_type",
    "name": "发布类型",
    "type": "radioV2",
    "required": true,
    "visible": true,
    "printable": true,
    "option": [
      {"value": "publish", "text": "发布"},
      {"value": "rollback", "text": "回滚"}
    ]
  },
  {
    "id": "site",
    "name": "发布站点",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true
  },
  {
    "id": "cluster",
    "name": "集群",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true
  },
  {
    "id": "namespace",
    "name": "命名空间",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true
  },
  {
    "id": "reason",
    "name": "发布说明",
    "type": "textarea",
    "required": true,
    "visible": true,
    "printable": true
  },
  {
    "id": "release_run_id",
    "name": "发布单ID",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true
  }
]`};function Kt(){const{t}=oe(),[n,s]=h.useState(!1),[c,p]=h.useState(!1),[x,_]=h.useState([]),[I,k]=h.useState(!1),[f,y]=h.useState(null),[r,l]=h.useState([]),[U,L]=h.useState(!1),[R,v]=h.useState([]),[j,C]=h.useState({open:!1,message:"",severity:"success"}),[o,i]=h.useState({name:"",type:"feishu",enabled:!0,app_id:"",app_secret:"",approval_code:"",process_code:"",template_id:"",form_fields:te.feishu,approver_user_ids:"",cc_user_ids:"",cc_open_ids:"",api_base_url:"",callback_url:""}),w=async()=>{s(!0);try{const a=await O.getApprovalConfig();console.log("加载到的工单配置列表：",a),_(a.configs||[])}catch(a){console.error("Failed to load approval configs:",a)}finally{s(!1)}},P=async a=>{L(!0);try{const S=(await dt.getUsersWithPagination({page:1,pageSize:100,keyword:a||void 0})).users||[];return l(S),S}catch(u){return console.error("Failed to load users:",u),l([]),[]}finally{L(!1)}};h.useEffect(()=>{w()},[]);const W=a=>{a?(console.log("编辑配置，原始数据：",a),y(a),i({name:a.name||"",type:a.type||"feishu",enabled:a.enabled!==void 0?a.enabled:!0,app_id:a.app_id||"",app_secret:a.app_secret||"",approval_code:a.approval_code||"",process_code:a.process_code||"",template_id:a.template_id||"",form_fields:a.form_fields||te[a.type||"feishu"],approver_user_ids:a.approver_user_ids||"",cc_user_ids:a.cc_user_ids||"",cc_open_ids:a.cc_open_ids||"",api_base_url:a.api_base_url||"",callback_url:a.callback_url?.trim()||$(a.type||"feishu")}),P().then(u=>{v(zt(a.approver_user_ids,u||[]))})):(y(null),v([]),i({name:"",type:"feishu",enabled:!0,app_id:"",app_secret:"",approval_code:"",process_code:"",template_id:"",form_fields:te.feishu,approver_user_ids:"",cc_user_ids:"",cc_open_ids:"",api_base_url:"",callback_url:$("feishu")})),k(!0)},A=()=>{k(!1),y(null),v([])},D=async()=>{try{try{JSON.parse(o.form_fields||"[]")}catch{C({open:!0,message:t("settings.approvalConfig.invalidJsonFormat"),severity:"error"});return}const a=R.find(S=>!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(S));if(a){Y(t("settings.approvalConfig.approverConfig.invalidEmail",{email:a}));return}const u={name:o.name,type:o.type,enabled:o.enabled,app_id:o.app_id,app_secret:o.app_secret,approval_code:o.approval_code,process_code:o.process_code,template_id:o.template_id,form_fields:o.form_fields,approver_user_ids:JSON.stringify(R),cc_user_ids:o.cc_user_ids,cc_open_ids:o.cc_open_ids,api_base_url:o.api_base_url,callback_url:o.callback_url?.trim()||$(o.type||"feishu")};f?(await O.updateApprovalConfig(f.id,u),C({open:!0,message:t("settings.approvalConfig.messages.updateSuccess"),severity:"success"})):(await O.updateApprovalConfig(null,u),C({open:!0,message:t("settings.approvalConfig.messages.createSuccess"),severity:"success"})),A(),w()}catch(a){console.error("Failed to save config:",a),C({open:!0,message:t("settings.approvalConfig.messages.saveFailed")+": "+a.message,severity:"error"})}},H=async()=>{if(!f?.id){Y(t("settings.approvalConfig.testFormDetailNeedSaved"));return}const a=o.type,u=(a==="feishu"||a==="lark"?o.approval_code:a==="dingtalk"?o.process_code:o.template_id)||"";if(!u.trim()){Y(t("settings.approvalConfig.testFormDetailNeedCode"));return}try{p(!0);const T=(await O.getApprovalFormDetail({approval_config_id:f.id,approval_code:u.trim(),platform:a})).form_fields,Oe=Array.isArray(T)?T.length:0;i({...o,form_fields:JSON.stringify(T,null,2)}),Ke(t("settings.approvalConfig.testFormDetailSuccess",{count:Oe}))}catch(S){const T=S?.message||t("settings.approvalConfig.testFormDetailFailed");Y(T)}finally{p(!1)}},ie=async a=>{if(await ct(t("settings.approvalConfig.deleteConfirm")))try{await O.deleteApprovalConfig(a),C({open:!0,message:t("settings.approvalConfig.messages.deleteSuccess"),severity:"success"}),w()}catch(u){console.error("Failed to delete config:",u),C({open:!0,message:t("settings.approvalConfig.messages.saveFailed")+": "+u.message,severity:"error"})}},ne=a=>{i({...o,type:a,form_fields:te[a],approval_code:"",process_code:"",template_id:""})},X=a=>({feishu:t("settings.approvalConfig.platforms.feishu"),lark:t("settings.approvalConfig.platforms.lark"),dingtalk:t("settings.approvalConfig.platforms.dingtalk"),wechat:t("settings.approvalConfig.platforms.wechat")})[a]||a,Q=a=>a?new Date(a).toLocaleString():"-",q=a=>{const u=a==="feishu"||a==="lark"?"feishu":a==="dingtalk"?"dingtalk":"wechat";return{title:t(`settings.approvalConfig.fieldGuide.${u}.title`),summary:t(`settings.approvalConfig.fieldGuide.${u}.summary`),examples:t(`settings.approvalConfig.fieldGuide.${u}.examples`,{returnObjects:!0}),notes:t(`settings.approvalConfig.fieldGuide.${u}.notes`,{returnObjects:!0})}};return n?e.jsx(g,{sx:{display:"flex",justifyContent:"center",py:4},children:e.jsx(et,{})}):e.jsxs(E,{spacing:4,children:[e.jsxs(g,{children:[e.jsxs(g,{sx:{display:"flex",justifyContent:"space-between",alignItems:"center",mb:3},children:[e.jsxs(g,{children:[e.jsxs(d,{variant:"h6",sx:{display:"flex",alignItems:"center"},children:[e.jsx(me,{sx:{mr:1}})," ",t("settings.approvalConfig.title")]}),e.jsx(d,{variant:"body2",color:"text.secondary",sx:{mt:.5},children:t("settings.approvalConfig.subtitle")})]}),x.length===0&&e.jsx(K,{variant:"contained",startIcon:e.jsx(tt,{}),onClick:()=>W(),children:t("settings.approvalConfig.addConfig")})]}),e.jsx(B,{})]}),e.jsxs(F,{severity:"info",icon:e.jsx(me,{}),children:[e.jsx(d,{variant:"body2",fontWeight:"bold",gutterBottom:!0,children:t("settings.approvalConfig.tips.title")}),e.jsxs(d,{variant:"body2",component:"div",children:["• ",e.jsx("strong",{children:t("settings.approvalConfig.platforms.feishu")}),": ",t("settings.approvalConfig.tips.feishu"),e.jsx("br",{}),"• ",e.jsx("strong",{children:t("settings.approvalConfig.platforms.lark")}),": ",t("settings.approvalConfig.tips.lark"),e.jsx("br",{}),"• ",e.jsx("strong",{children:t("settings.approvalConfig.platforms.dingtalk")}),": ",t("settings.approvalConfig.tips.dingtalk"),e.jsx("br",{}),"• ",e.jsx("strong",{children:t("settings.approvalConfig.platforms.wechat")}),": ",t("settings.approvalConfig.tips.wechat")]})]}),x.length===0?e.jsx(F,{severity:"info",children:t("settings.approvalConfig.noConfigs")}):e.jsx(Se,{component:z,variant:"outlined",children:e.jsxs(Te,{children:[e.jsx(Ie,{children:e.jsxs(G,{children:[e.jsx(b,{children:t("settings.approvalConfig.table.name")}),e.jsx(b,{children:t("settings.approvalConfig.table.platform")}),e.jsx(b,{children:t("settings.approvalConfig.table.appId")}),e.jsx(b,{children:t("settings.approvalConfig.table.flowCode")}),e.jsx(b,{align:"center",children:t("settings.approvalConfig.table.status")}),e.jsx(b,{align:"center",children:t("settings.approvalConfig.table.createdAt")}),e.jsx(b,{align:"right",children:t("settings.approvalConfig.table.actions")})]})}),e.jsx(Ue,{children:x.map(a=>e.jsxs(G,{children:[e.jsx(b,{children:e.jsx(g,{sx:{display:"flex",alignItems:"center",gap:1},children:a.name})}),e.jsx(b,{children:e.jsx(ee,{label:X(a.type),size:"small",variant:"outlined",color:a.type==="feishu"||a.type==="lark"?"primary":a.type==="dingtalk"?"info":"secondary"})}),e.jsx(b,{children:e.jsx(d,{variant:"body2",sx:{fontFamily:"monospace"},children:a.app_id})}),e.jsx(b,{children:e.jsx(d,{variant:"body2",sx:{fontFamily:"monospace"},children:a.approval_code||a.process_code||a.template_id||"-"})}),e.jsx(b,{align:"center",children:a.enabled?e.jsx(ee,{icon:e.jsx(ze,{}),label:t("common.enabled"),size:"small",color:"success"}):e.jsx(ee,{icon:e.jsx(st,{}),label:t("common.disabled"),size:"small",color:"default"})}),e.jsx(b,{align:"center",children:e.jsx(d,{variant:"body2",color:"text.secondary",children:Q(a.created_at)})}),e.jsx(b,{align:"right",children:e.jsxs(g,{sx:{display:"flex",gap:.5,justifyContent:"flex-end"},children:[e.jsx(we,{title:t("common.edit"),children:e.jsx(ke,{size:"small",color:"info",onClick:()=>W(a),children:e.jsx(at,{})})}),e.jsx(we,{title:t("common.delete"),children:e.jsx(ke,{size:"small",color:"error",onClick:()=>ie(a.id),children:e.jsx(rt,{})})})]})})]},a.id))})]})}),e.jsxs(ot,{open:I,onClose:A,maxWidth:"md",fullWidth:!0,children:[e.jsx(it,{children:t(f?"settings.approvalConfig.editConfig":"settings.approvalConfig.addConfig")}),e.jsx(nt,{children:e.jsxs(E,{spacing:3,sx:{mt:1},children:[e.jsx(d,{variant:"subtitle1",fontWeight:"600",children:t("settings.approvalConfig.basicConfig")}),e.jsx(m,{fullWidth:!0,label:t("settings.approvalConfig.configName"),value:o.name,onChange:a=>i({...o,name:a.target.value}),required:!0}),e.jsxs(he,{fullWidth:!0,children:[e.jsx(Me,{children:t("settings.approvalConfig.platformType")}),e.jsxs(Ee,{value:o.type,label:t("settings.approvalConfig.platformType"),onChange:a=>ne(a.target.value),children:[e.jsx(J,{value:"feishu",children:t("settings.approvalConfig.platforms.feishu")}),e.jsx(J,{value:"lark",children:t("settings.approvalConfig.platforms.lark")}),e.jsx(J,{value:"dingtalk",children:t("settings.approvalConfig.platforms.dingtalk")}),e.jsx(J,{value:"wechat",children:t("settings.approvalConfig.platforms.wechat")})]})]}),e.jsx(m,{fullWidth:!0,label:t("settings.approvalConfig.appId"),value:o.app_id,onChange:a=>i({...o,app_id:a.target.value}),required:!0,helperText:t("settings.approvalConfig.appIdHelper")}),e.jsx(m,{fullWidth:!0,label:t("settings.approvalConfig.appSecret"),type:"password",value:o.app_secret,onChange:a=>i({...o,app_secret:a.target.value}),required:!0,helperText:t(f?"settings.approvalConfig.appSecretHelperEdit":"settings.approvalConfig.appSecretHelper"),placeholder:f?t("settings.approvalConfig.appSecretPlaceholder"):""}),(o.type==="feishu"||o.type==="lark")&&e.jsx(m,{fullWidth:!0,label:t("settings.approvalConfig.approvalCode"),value:o.approval_code,onChange:a=>i({...o,approval_code:a.target.value}),required:!0,helperText:t("settings.approvalConfig.approvalCodeHelper")}),o.type==="dingtalk"&&e.jsx(m,{fullWidth:!0,label:t("settings.approvalConfig.processCode"),value:o.process_code,onChange:a=>i({...o,process_code:a.target.value}),required:!0,helperText:t("settings.approvalConfig.processCodeHelper")}),o.type==="wechat"&&e.jsx(m,{fullWidth:!0,label:t("settings.approvalConfig.templateId"),value:o.template_id,onChange:a=>i({...o,template_id:a.target.value}),required:!0,helperText:t("settings.approvalConfig.templateIdHelper")}),e.jsx(F,{severity:"info",children:t("settings.approvalConfig.testSectionHint")}),e.jsx(g,{sx:{display:"flex",gap:2,justifyContent:"flex-start"},children:e.jsx(K,{variant:"outlined",onClick:H,disabled:c,children:t(c?"common.testing":"settings.approvalConfig.testFormDetail")})}),e.jsx(B,{}),e.jsxs(pe,{children:[e.jsx(de,{expandIcon:e.jsx(le,{}),children:e.jsx(d,{variant:"subtitle1",fontWeight:"600",children:t("settings.approvalConfig.apiConfig.title")})}),e.jsx(ce,{children:e.jsxs(E,{spacing:2,children:[e.jsx(m,{fullWidth:!0,label:t("settings.approvalConfig.apiConfig.apiBaseUrl"),value:o.api_base_url||"",onChange:a=>i({...o,api_base_url:a.target.value}),helperText:t("settings.approvalConfig.apiConfig.apiBaseUrlHelper"),placeholder:"https://open.larksuite.com/open-apis"}),e.jsxs(g,{children:[e.jsx(m,{fullWidth:!0,label:t("settings.approvalConfig.apiConfig.callbackUrl"),value:o.callback_url||"",onChange:a=>i({...o,callback_url:a.target.value}),placeholder:$(o.type||"feishu"),helperText:t("settings.approvalConfig.apiConfig.callbackUrlHelper")}),e.jsx(g,{sx:{mt:1},children:e.jsx(K,{size:"small",variant:"outlined",onClick:()=>{i({...o,callback_url:$(o.type||"feishu")})},children:t("settings.approvalConfig.apiConfig.autoGenerateCallbackUrl")})})]})]})})]}),e.jsxs(pe,{children:[e.jsx(de,{expandIcon:e.jsx(le,{}),children:e.jsx(d,{variant:"subtitle1",fontWeight:"600",children:t("settings.approvalConfig.approverConfig.title")})}),e.jsx(ce,{children:e.jsx(E,{spacing:2,children:e.jsx(xt,{multiple:!0,freeSolo:!0,options:r.map(a=>a.email).filter(Boolean),value:R,onChange:(a,u)=>{const S=u.map(T=>(typeof T=="string"?T:"").trim()).filter(T=>T.length>0);v(S)},onInputChange:(a,u)=>{u?P(u):P()},loading:U,filterOptions:(a,u)=>{const S=u.inputValue.trim().toLowerCase();return S?a.filter(T=>T.toLowerCase().includes(S)):a},renderInput:a=>e.jsx(m,{...a,label:t("settings.approvalConfig.approverConfig.approverEmails"),placeholder:t("settings.approvalConfig.approverConfig.approverEmailsPlaceholder"),helperText:t("settings.approvalConfig.approverConfig.approverEmailsHelper")}),renderTags:(a,u)=>a.map((S,T)=>h.createElement(ee,{...u({index:T}),key:S,label:S}))})})})]}),e.jsxs(pe,{defaultExpanded:!0,children:[e.jsx(de,{expandIcon:e.jsx(le,{}),children:e.jsx(d,{variant:"subtitle1",fontWeight:"600",children:t("settings.approvalConfig.formFields")})}),e.jsx(ce,{children:e.jsxs(E,{spacing:2,children:[e.jsxs(F,{severity:"info",children:[e.jsx(d,{variant:"body2",children:t("settings.approvalConfig.formFieldsHelper")}),e.jsx(d,{variant:"body2",fontWeight:600,gutterBottom:!0,sx:{mt:1.5},children:t("settings.approvalConfig.buildMasterKeywordTitle")}),e.jsx(d,{variant:"caption",color:"text.secondary",display:"block",sx:{mb:1},children:t("settings.approvalConfig.buildMasterKeywordIntro")}),e.jsx(d,{variant:"caption",color:"text.secondary",display:"block",sx:{mb:1.5},children:t("settings.approvalConfig.buildMasterKeywordNote")}),e.jsx(Se,{component:z,variant:"outlined",sx:{maxHeight:420,bgcolor:"background.paper"},children:e.jsxs(Te,{size:"small",stickyHeader:!0,children:[e.jsx(Ie,{children:e.jsxs(G,{children:[e.jsx(b,{sx:{fontWeight:600,minWidth:140},children:t("settings.approvalConfig.buildMasterKwTable.keyword")}),e.jsx(b,{sx:{fontWeight:600,minWidth:160},children:t("settings.approvalConfig.buildMasterKwTable.source")}),e.jsx(b,{sx:{fontWeight:600,minWidth:200},children:t("settings.approvalConfig.buildMasterKwTable.value")}),e.jsx(b,{sx:{fontWeight:600,minWidth:140},children:t("settings.approvalConfig.buildMasterKwTable.widget")})]})}),e.jsx(Ue,{children:Et.map(a=>e.jsxs(h.Fragment,{children:[e.jsx(G,{children:e.jsx(b,{colSpan:4,sx:{bgcolor:"action.hover",fontWeight:600,fontSize:"0.75rem",py:.75},children:t(`settings.approvalConfig.buildMasterKwGroups.${a.groupKey}`)})}),a.rows.map(u=>e.jsxs(G,{hover:!0,children:[e.jsx(b,{sx:{fontSize:"0.75rem",fontFamily:"monospace",verticalAlign:"top"},children:e.jsx("code",{children:u.keyword})}),e.jsx(b,{sx:{fontSize:"0.75rem",verticalAlign:"top"},children:t(`settings.approvalConfig.buildMasterKwRows.${u.rowKey}.source`)}),e.jsx(b,{sx:{fontSize:"0.75rem",verticalAlign:"top",color:"text.secondary"},children:t(`settings.approvalConfig.buildMasterKwRows.${u.rowKey}.value`)}),e.jsx(b,{sx:{fontSize:"0.75rem",verticalAlign:"top"},children:t(`settings.approvalConfig.buildMasterKwRows.${u.rowKey}.widget`)})]},u.rowKey))]},a.groupKey))})]})}),e.jsx(d,{variant:"caption",color:"text.secondary",display:"block",sx:{mt:1.5,whiteSpace:"pre-wrap"},children:t("settings.approvalConfig.buildMasterKeywordExample")})]}),e.jsxs(F,{severity:"warning",children:[e.jsx(d,{variant:"body2",fontWeight:600,gutterBottom:!0,children:t("settings.approvalConfig.fieldGuide.title")}),e.jsx(d,{variant:"body2",gutterBottom:!0,children:q(o.type).title}),e.jsx(d,{variant:"body2",color:"text.secondary",gutterBottom:!0,children:q(o.type).summary}),q(o.type).examples.map(a=>e.jsxs(d,{variant:"body2",children:["• ",a]},a)),q(o.type).notes.map(a=>e.jsx(d,{variant:"caption",color:"text.secondary",display:"block",children:a},a))]}),e.jsx(m,{fullWidth:!0,label:t("settings.approvalConfig.formFieldsJson"),value:o.form_fields,onChange:a=>i({...o,form_fields:a.target.value}),multiline:!0,rows:12,required:!0,sx:{fontFamily:"monospace","& textarea":{fontFamily:"monospace",fontSize:"0.875rem"}}})]})})]}),e.jsx(M,{control:e.jsx(re,{checked:o.enabled,onChange:a=>i({...o,enabled:a.target.checked})}),label:t("settings.approvalConfig.enableConfig")})]})}),e.jsxs(lt,{children:[e.jsx(K,{onClick:A,children:t("common.cancel")}),e.jsx(K,{variant:"contained",onClick:D,disabled:!o.name||!o.app_id||!o.app_secret,children:t(f?"common.save":"common.create")})]})]}),e.jsx(pt,{open:j.open,autoHideDuration:4e3,onClose:()=>C({...j,open:!1}),anchorOrigin:{vertical:"top",horizontal:"center"},children:e.jsx(F,{severity:j.severity,onClose:()=>C({...j,open:!1}),children:j.message})})]})}function Bt(t){const n=["oidc","feishu","lark","dingtalk","wecom"],s=(t||"").trim().toLowerCase();return n.includes(s)?s:s.includes("feishu")||s.includes("飞书")?"feishu":s.includes("lark")?"lark":s.includes("ding")||s.includes("钉钉")?"dingtalk":s.includes("wecom")||s.includes("wework")||s.includes("企微")?"wecom":"oidc"}function V(t){const{children:n,value:s,index:c,...p}=t;return e.jsx("div",{role:"tabpanel",hidden:s!==c,id:`settings-tabpanel-${c}`,"aria-labelledby":`settings-tab-${c}`,...p,children:s===c&&e.jsx(g,{sx:{py:3},children:n})})}function as(){const{t}=oe(),{refreshSettings:n}=qe(),[s,c]=h.useState(0),[p,x]=h.useState(!1),[_,I]=h.useState(!0),[k,f]=h.useState(!1),[y,r]=h.useState({siteName:"KeyOps",showWatermark:!1}),[l,U]=h.useState({authMethod:"password",passwordLogin:{enabled:!0,passwordMinLength:8,passwordComplexity:!0,sessionTimeout:30},sso:{enabled:!1,provider:"oidc",clientId:"",clientSecret:"",authUrl:"",tokenUrl:"",userInfoUrl:"",redirectUrl:"",scopes:"openid email profile"},ldap:{enabled:!1,server:"",port:389,bindDn:"",bindPassword:"",baseDn:"",userFilter:"(uid={username})",useTLS:!1}}),L=async()=>{try{I(!0);const v=await _e.getAllSettings(),j={};Array.isArray(v)?v.forEach(i=>{j[i.category]||(j[i.category]={}),j[i.category][i.key]=i.value}):typeof v=="object"&&Object.keys(v).forEach(i=>{const w=v[i];typeof w=="object"&&(j[i]=w)});const C=i=>{if(typeof i!="string")return i;if(i==="true")return!0;if(i==="false")return!1;if(/^\d+$/.test(i)){const w=parseInt(i,10);if(!isNaN(w))return w}if(/^\d+\.\d+$/.test(i)){const w=parseFloat(i);if(!isNaN(w))return w}return i},o=(i,w)=>{const P={};for(const[W,A]of Object.entries(i)){let D=W;w&&W.startsWith(`${w}.`)&&(D=W.substring(w.length+1)),P[D]=C(A)}return P};if(j.system&&r(i=>({...i,...o(j.system,"system")})),j.auth){const i=o(j.auth),w={authMethod:i.authMethod||"password",passwordLogin:{enabled:i.passwordLoginEnabled??!0,passwordMinLength:i.passwordMinLength||8,passwordComplexity:i.passwordComplexity??!0,sessionTimeout:i.passwordSessionTimeout||30},sso:{enabled:i.ssoEnabled??!1,provider:Bt(i.ssoProvider),clientId:i.ssoClientId||"",clientSecret:i.ssoClientSecret||"",authUrl:i.ssoAuthUrl||"",tokenUrl:i.ssoTokenUrl||"",userInfoUrl:i.ssoUserInfoUrl||"",redirectUrl:i.ssoRedirectUrl||"",scopes:i.ssoScopes||"openid email profile"},ldap:{enabled:i.ldapEnabled??!1,server:i.ldapServer||"",port:i.ldapPort||389,bindDn:i.ldapBindDn||"",bindPassword:i.ldapBindPassword||"",baseDn:i.ldapBaseDn||"",userFilter:i.ldapUserFilter||"(uid={username})",useTLS:i.ldapUseTLS??!1}};U(w)}}catch(v){console.error("Failed to load settings:",v)}finally{I(!1)}};h.useEffect(()=>{L()},[]);const R=async()=>{try{f(!0);const v={authMethod:l.authMethod,passwordLoginEnabled:l.passwordLogin.enabled,passwordMinLength:l.passwordLogin.passwordMinLength,passwordComplexity:l.passwordLogin.passwordComplexity,passwordSessionTimeout:l.passwordLogin.sessionTimeout,ssoEnabled:l.sso.enabled,ssoProvider:l.sso.provider,ssoClientId:l.sso.clientId,ssoClientSecret:l.sso.clientSecret,ssoAuthUrl:l.sso.authUrl,ssoTokenUrl:l.sso.tokenUrl,ssoUserInfoUrl:l.sso.userInfoUrl,ssoRedirectUrl:l.sso.redirectUrl,ssoScopes:l.sso.scopes,ldapEnabled:l.ldap.enabled,ldapServer:l.ldap.server,ldapPort:l.ldap.port,ldapBindDn:l.ldap.bindDn,ldapBindPassword:l.ldap.bindPassword,ldapBaseDn:l.ldap.baseDn,ldapUserFilter:l.ldap.userFilter,ldapUseTLS:l.ldap.useTLS},j={system:y,auth:v};await _e.updateSettings(j),x(!0),Ke(t("settings.saveSuccess")),setTimeout(()=>x(!1),3e3),await n(),await L()}catch(v){console.error("Failed to save settings:",v),Y(t("settings.saveFailed")+": "+v.message)}finally{f(!1)}};return e.jsxs(g,{children:[e.jsxs(g,{sx:{display:"flex",alignItems:"center",mb:3},children:[e.jsx(ut,{sx:{fontSize:40,mr:2,color:"primary.main"}}),e.jsxs(g,{children:[e.jsx(d,{variant:"h4",component:"h1",fontWeight:"700",children:t("settings.title")}),e.jsx(d,{variant:"body2",color:"text.secondary",children:t("settings.adminConfig")})]})]}),_&&e.jsx(F,{severity:"info",sx:{mb:3},children:t("settings.loadingConfig")}),p&&e.jsx(F,{severity:"success",sx:{mb:3},icon:e.jsx(ze,{}),children:t("settings.saveSuccessRestart")}),e.jsxs(z,{sx:{borderRadius:2,overflow:"hidden"},children:[e.jsxs(ht,{value:s,onChange:(v,j)=>c(j),variant:"scrollable",scrollButtons:"auto",sx:{borderBottom:1,borderColor:"divider",bgcolor:"background.paper","& .MuiTab-root":{minHeight:64,mx:1}},children:[e.jsx(N,{icon:e.jsx(ae,{}),label:t("settings.systemConfig")}),e.jsx(N,{icon:e.jsx(ge,{}),label:t("settings.authConfig")}),e.jsx(N,{icon:e.jsx(fe,{}),label:t("settings.twoFactor.title")}),e.jsx(N,{icon:e.jsx(me,{}),label:t("settings.approvalConfigTab")}),e.jsx(N,{icon:e.jsx(gt,{}),label:"mcp密钥"})]}),e.jsxs(g,{sx:{px:3,maxWidth:1200},children:[e.jsx(V,{value:s,index:0,children:e.jsx(qt,{config:y,onChange:r})}),e.jsx(V,{value:s,index:1,children:e.jsx(Mt,{config:l,onChange:U})}),e.jsx(V,{value:s,index:2,children:e.jsx(mt,{showGlobalConfig:!0})}),e.jsx(V,{value:s,index:3,children:e.jsx(Kt,{})}),e.jsx(V,{value:s,index:4,children:e.jsx(vt,{})})]}),[0,1].includes(s)&&e.jsx(g,{sx:{px:3,pb:3,display:"flex",justifyContent:"flex-end",gap:2},children:e.jsx(K,{variant:"contained",size:"large",startIcon:e.jsx(ft,{}),onClick:R,disabled:k,children:t(k?"common.saving":"common.save")})})]})]})}export{as as default};
