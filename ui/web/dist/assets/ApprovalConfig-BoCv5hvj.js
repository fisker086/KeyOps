import{u as ee,r as n,j as e,B as p,C as ae,T as o,dC as L,e as v,V as le,a7 as z,A,P as se,G as te,av as ie,d as B,I as P,W as re,a9 as oe,X as ne,Y as pe,Z as de,t as d,F as ce,p as ue,q as ge,s as F,O as x,af as m,a1 as fe,$ as he,cm as C,cp as ve,ac as W,x as _e}from"./index-DbKJwvK5.js";import{S as _}from"./Stack-Bognyuj_.js";import{T as xe}from"./TableContainer-Cwv0kZgL.js";import{T as me,a as Ce,b as J,c as r,d as be}from"./TableRow-DqxulK_b.js";import{C as b}from"./Chip-B3STCJQ6.js";import{A as j,a as y,b as w}from"./AccordionSummary-B67OQCKa.js";import{A as je}from"./Autocomplete-jUE2gKY2.js";import{S as ye}from"./Switch-B4hIX1rE.js";const u={feishu:`[
  {
    "id": "widget17424405077590001",
    "name": "工单标题",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true,
    "enable_default_value": false,
    "default_value_type": "",
    "widget_default_value": "",
    "display_condition": null
  },
  {
    "id": "widget17611283640360001",
    "name": "详细描述",
    "type": "textarea",
    "required": false,
    "visible": true,
    "printable": true,
    "enable_default_value": false,
    "default_value_type": "",
    "widget_default_value": "",
    "display_condition": null
  },
  {
    "id": "widget17611425102270001",
    "name": "申请类型",
    "type": "radioV2",
    "required": false,
    "visible": true,
    "printable": true,
    "enable_default_value": false,
    "default_value_type": "",
    "widget_default_value": "",
    "display_condition": null,
    "option": [
      {"value": "mh22s1w3-nkqsak2eis-0", "text": "host_access"},
      {"value": "mh22s1w3-dipw4j04vb-0", "text": "host_group_access"}
    ]
  },
  {
    "id": "widget17611283900470001",
    "name": "申请理由",
    "type": "textarea",
    "required": false,
    "visible": true,
    "printable": true,
    "enable_default_value": false,
    "default_value_type": "",
    "widget_default_value": "",
    "display_condition": null
  },
  {
    "id": "widget17611284241860001",
    "name": "申请资源",
    "type": "textarea",
    "required": false,
    "visible": true,
    "printable": true,
    "enable_default_value": false,
    "default_value_type": "",
    "widget_default_value": "",
    "display_condition": null
  },
  {
    "id": "widget17611477809060001",
    "name": "权限时长",
    "type": "input",
    "required": false,
    "visible": true,
    "printable": true,
    "enable_default_value": false,
    "default_value_type": "",
    "widget_default_value": "",
    "display_condition": null
  }
]`,dingtalk:`[
  {
    "name": "title",
    "label": "工单标题",
    "field": "title"
  },
  {
    "name": "description",
    "label": "详细描述",
    "field": "description"
  },
  {
    "name": "type",
    "label": "申请类型",
    "field": "type",
    "options": [
      {"value": "host_access", "label": "主机访问权限"},
      {"value": "host_group_access", "label": "主机组访问权限"}
    ]
  },
  {
    "name": "reason",
    "label": "申请理由",
    "field": "reason"
  },
  {
    "name": "duration",
    "label": "权限时长",
    "field": "duration"
  }
]`,wechat:`[
  {
    "control": "Text",
    "id": "Text-title",
    "label": "工单标题",
    "field": "title"
  },
  {
    "control": "Textarea",
    "id": "Textarea-description",
    "label": "详细描述",
    "field": "description"
  },
  {
    "control": "Select",
    "id": "Select-type",
    "label": "申请类型",
    "field": "type",
    "options": [
      {"value": "host_access", "label": "主机访问权限"},
      {"value": "host_group_access", "label": "主机组访问权限"}
    ]
  },
  {
    "control": "Textarea",
    "id": "Textarea-reason",
    "label": "申请理由",
    "field": "reason"
  },
  {
    "control": "Text",
    "id": "Text-duration",
    "label": "权限时长",
    "field": "duration"
  }
]`};function Ue(){const{t:l}=ee(),[N,U]=n.useState(!1),[k,$]=n.useState([]),[M,q]=n.useState(!1),[g,T]=n.useState(null),[D,R]=n.useState([]),[G,E]=n.useState(!1),[O,f]=n.useState([]),[s,i]=n.useState({name:"",type:"feishu",enabled:!0,app_id:"",app_secret:"",approval_code:"",process_code:"",template_id:"",form_fields:u.feishu,approver_user_ids:"",cc_user_ids:"",cc_open_ids:"",api_base_url:"",callback_url:""}),S=async()=>{U(!0);try{const a=await C.getApprovalConfig();console.log("加载到的工单配置列表：",a),$(a.configs||[])}catch(a){console.error("Failed to load approval configs:",a)}finally{U(!1)}};n.useEffect(()=>{S()},[]),n.useEffect(()=>{(async()=>{E(!0);try{const t=await ve.getUsersWithPagination({page:1,pageSize:200});R(t.users||[])}catch(t){console.error("Failed to load users:",t)}finally{E(!1)}})()},[]);const H=a=>{if(a)if(console.log("编辑配置，原始数据：",a),T(a),i({name:a.name||"",type:a.type||"feishu",enabled:a.enabled!==void 0?a.enabled:!0,app_id:a.app_id||"",app_secret:a.app_secret||"",approval_code:a.approval_code||"",process_code:a.process_code||"",template_id:a.template_id||"",form_fields:a.form_fields||u[a.type||"feishu"],approver_user_ids:a.approver_user_ids||"",cc_user_ids:a.cc_user_ids||"",cc_open_ids:a.cc_open_ids||"",api_base_url:a.api_base_url||"",callback_url:a.callback_url||""}),a.approver_user_ids)try{const t=JSON.parse(a.approver_user_ids);if(Array.isArray(t)){const c=t.map(h=>D.find(V=>V.email===h)).filter(h=>h!==void 0);f(c)}}catch{f([])}else f([]);else T(null),i({name:"",type:"feishu",enabled:!0,app_id:"",app_secret:"",approval_code:"",process_code:"",template_id:"",form_fields:u.feishu,approver_user_ids:"",cc_user_ids:"",cc_open_ids:"",api_base_url:"",callback_url:""}),f([]);q(!0)},I=()=>{q(!1),T(null),f([])},X=async()=>{try{try{JSON.parse(s.form_fields||"[]")}catch{W(l("settings.approvalConfig.invalidJsonFormat"));return}const a={name:s.name,type:s.type,enabled:s.enabled,app_id:s.app_id,app_secret:s.app_secret,approval_code:s.approval_code,process_code:s.process_code,template_id:s.template_id,form_fields:s.form_fields,approver_user_ids:JSON.stringify(O.map(t=>t.email)),cc_user_ids:s.cc_user_ids,cc_open_ids:s.cc_open_ids,api_base_url:s.api_base_url,callback_url:s.callback_url};g?(await C.updateApprovalConfig(g.id,a),m(l("settings.approvalConfig.messages.updateSuccess"))):(await C.updateApprovalConfig(null,a),m(l("settings.approvalConfig.messages.createSuccess"))),I(),S()}catch(a){console.error("Failed to save config:",a),W(l("settings.approvalConfig.messages.saveFailed")+": "+a.message)}},Y=async a=>{if(await _e(l("settings.approvalConfig.deleteConfirm")))try{await C.deleteApprovalConfig(a),m(l("settings.approvalConfig.messages.deleteSuccess")),S()}catch(t){console.error("Failed to delete config:",t),W(l("settings.approvalConfig.messages.saveFailed")+": "+t.message)}},Z=a=>{i({...s,type:a,form_fields:u[a],approval_code:"",process_code:"",template_id:""})},K=a=>({feishu:l("settings.approvalConfig.platforms.feishu"),dingtalk:l("settings.approvalConfig.platforms.dingtalk"),wechat:l("settings.approvalConfig.platforms.wechat")})[a]||a,Q=a=>a?new Date(a).toLocaleString():"-";return N?e.jsx(p,{sx:{display:"flex",justifyContent:"center",py:4},children:e.jsx(ae,{})}):e.jsx(p,{sx:{p:3},children:e.jsxs(_,{spacing:4,children:[e.jsxs(p,{children:[e.jsxs(p,{sx:{display:"flex",justifyContent:"space-between",alignItems:"center",mb:3},children:[e.jsxs(p,{children:[e.jsxs(o,{variant:"h5",fontWeight:600,sx:{display:"flex",alignItems:"center"},children:[e.jsx(L,{sx:{mr:1}})," ",l("settings.approvalConfig.title")]}),e.jsx(o,{variant:"body2",color:"text.secondary",sx:{mt:.5},children:l("settings.approvalConfig.subtitle")})]}),k.length===0&&e.jsx(v,{variant:"contained",startIcon:e.jsx(le,{}),onClick:()=>H(),children:l("settings.approvalConfig.addConfig")})]}),e.jsx(z,{})]}),e.jsxs(A,{severity:"info",icon:e.jsx(L,{}),children:[e.jsx(o,{variant:"body2",fontWeight:"bold",gutterBottom:!0,children:l("settings.approvalConfig.tips.title")}),e.jsxs(o,{variant:"body2",component:"div",children:["• ",e.jsx("strong",{children:l("settings.approvalConfig.platforms.feishu")}),": ",l("settings.approvalConfig.tips.feishu"),e.jsx("br",{}),"• ",e.jsx("strong",{children:l("settings.approvalConfig.platforms.dingtalk")}),": ",l("settings.approvalConfig.tips.dingtalk"),e.jsx("br",{}),"• ",e.jsx("strong",{children:l("settings.approvalConfig.platforms.wechat")}),": ",l("settings.approvalConfig.tips.wechat")]})]}),k.length===0?e.jsx(A,{severity:"info",children:l("settings.approvalConfig.noConfigs")}):e.jsx(xe,{component:se,variant:"outlined",children:e.jsxs(me,{children:[e.jsx(Ce,{children:e.jsxs(J,{children:[e.jsx(r,{children:l("settings.approvalConfig.table.name")}),e.jsx(r,{children:l("settings.approvalConfig.table.platform")}),e.jsx(r,{children:l("settings.approvalConfig.table.appId")}),e.jsx(r,{children:l("settings.approvalConfig.table.flowCode")}),e.jsx(r,{align:"center",children:l("settings.approvalConfig.table.status")}),e.jsx(r,{align:"center",children:l("settings.approvalConfig.table.createdAt")}),e.jsx(r,{align:"right",children:l("settings.approvalConfig.table.actions")})]})}),e.jsx(be,{children:k.map(a=>e.jsxs(J,{children:[e.jsx(r,{children:e.jsx(p,{sx:{display:"flex",alignItems:"center",gap:1},children:a.name})}),e.jsx(r,{children:e.jsx(b,{label:K(a.type),size:"small",variant:"outlined",color:a.type==="feishu"?"primary":a.type==="dingtalk"?"info":"secondary"})}),e.jsx(r,{children:e.jsx(o,{variant:"body2",sx:{fontFamily:"monospace"},children:a.app_id})}),e.jsx(r,{children:e.jsx(o,{variant:"body2",sx:{fontFamily:"monospace"},children:a.approval_code||a.process_code||a.template_id||"-"})}),e.jsx(r,{align:"center",children:a.enabled?e.jsx(b,{icon:e.jsx(te,{}),label:l("common.enabled"),size:"small",color:"success"}):e.jsx(b,{icon:e.jsx(ie,{}),label:l("common.disabled"),size:"small",color:"default"})}),e.jsx(r,{align:"center",children:e.jsx(o,{variant:"body2",color:"text.secondary",children:Q(a.created_at)})}),e.jsx(r,{align:"right",children:e.jsxs(p,{sx:{display:"flex",gap:.5,justifyContent:"flex-end"},children:[e.jsx(B,{title:l("common.edit"),children:e.jsx(P,{size:"small",color:"info",onClick:()=>H(a),children:e.jsx(re,{})})}),e.jsx(B,{title:l("common.delete"),children:e.jsx(P,{size:"small",color:"error",onClick:()=>Y(a.id),children:e.jsx(oe,{})})})]})})]},a.id))})]})}),e.jsxs(ne,{open:M,onClose:I,maxWidth:"md",fullWidth:!0,children:[e.jsx(pe,{children:l(g?"settings.approvalConfig.editConfig":"settings.approvalConfig.addConfig")}),e.jsx(de,{children:e.jsxs(_,{spacing:3,sx:{mt:1},children:[e.jsx(o,{variant:"subtitle1",fontWeight:"600",children:l("settings.approvalConfig.basicConfig")}),e.jsx(d,{fullWidth:!0,label:l("settings.approvalConfig.configName"),value:s.name,onChange:a=>i({...s,name:a.target.value}),required:!0}),e.jsxs(ce,{fullWidth:!0,children:[e.jsx(ue,{children:l("settings.approvalConfig.platformType")}),e.jsxs(ge,{value:s.type,label:l("settings.approvalConfig.platformType"),onChange:a=>Z(a.target.value),children:[e.jsx(F,{value:"feishu",children:l("settings.approvalConfig.platforms.feishu")}),e.jsx(F,{value:"dingtalk",children:l("settings.approvalConfig.platforms.dingtalk")}),e.jsx(F,{value:"wechat",children:l("settings.approvalConfig.platforms.wechat")})]})]}),e.jsx(d,{fullWidth:!0,label:l("settings.approvalConfig.appId"),value:s.app_id,onChange:a=>i({...s,app_id:a.target.value}),required:!0,helperText:l("settings.approvalConfig.appIdHelper")}),e.jsx(d,{fullWidth:!0,label:l("settings.approvalConfig.appSecret"),type:"password",value:s.app_secret,onChange:a=>i({...s,app_secret:a.target.value}),required:!0,helperText:l(g?"settings.approvalConfig.appSecretHelperEdit":"settings.approvalConfig.appSecretHelper"),placeholder:g?l("settings.approvalConfig.appSecretPlaceholder"):""}),s.type==="feishu"&&e.jsx(d,{fullWidth:!0,label:l("settings.approvalConfig.approvalCode"),value:s.approval_code,onChange:a=>i({...s,approval_code:a.target.value}),required:!0,helperText:l("settings.approvalConfig.approvalCodeHelper")}),s.type==="dingtalk"&&e.jsx(d,{fullWidth:!0,label:l("settings.approvalConfig.processCode"),value:s.process_code,onChange:a=>i({...s,process_code:a.target.value}),required:!0,helperText:l("settings.approvalConfig.processCodeHelper")}),s.type==="wechat"&&e.jsx(d,{fullWidth:!0,label:l("settings.approvalConfig.templateId"),value:s.template_id,onChange:a=>i({...s,template_id:a.target.value}),required:!0,helperText:l("settings.approvalConfig.templateIdHelper")}),e.jsx(z,{}),e.jsxs(j,{children:[e.jsx(y,{expandIcon:e.jsx(x,{}),children:e.jsx(o,{variant:"subtitle1",fontWeight:"600",children:l("settings.approvalConfig.apiConfig.title")})}),e.jsx(w,{children:e.jsxs(_,{spacing:2,children:[e.jsx(d,{fullWidth:!0,label:l("settings.approvalConfig.apiConfig.apiBaseUrl"),value:s.api_base_url||"",onChange:a=>i({...s,api_base_url:a.target.value}),helperText:l("settings.approvalConfig.apiConfig.apiBaseUrlHelper"),placeholder:"https://open.larksuite.com/open-apis"}),e.jsxs(p,{children:[e.jsx(d,{fullWidth:!0,label:l("settings.approvalConfig.apiConfig.callbackUrl"),value:s.callback_url||"",onChange:a=>i({...s,callback_url:a.target.value}),helperText:l("settings.approvalConfig.apiConfig.callbackUrlHelper"),placeholder:"https://your-domain.com/api/approvals/callback/feishu"}),e.jsxs(p,{sx:{mt:1,display:"flex",gap:1},children:[e.jsx(v,{size:"small",variant:"outlined",onClick:()=>{const a=window.location.origin,t=`/api/approvals/callback/ticket/${s.type}`;i({...s,callback_url:a+t})},children:l("settings.approvalConfig.apiConfig.autoGenerateCallbackUrl")}),e.jsx(v,{size:"small",variant:"outlined",color:"info",onClick:()=>{s.callback_url&&(navigator.clipboard.writeText(s.callback_url),m(l("settings.approvalConfig.apiConfig.callbackUrlCopied")))},disabled:!s.callback_url,children:l("settings.approvalConfig.apiConfig.copyCallbackUrl")})]})]})]})})]}),e.jsxs(j,{children:[e.jsx(y,{expandIcon:e.jsx(x,{}),children:e.jsx(o,{variant:"subtitle1",fontWeight:"600",children:l("settings.approvalConfig.approverConfig.title")})}),e.jsx(w,{children:e.jsx(_,{spacing:2,children:e.jsx(je,{multiple:!0,options:D,getOptionLabel:a=>`${a.email} (${a.username||""})`,value:O,onChange:(a,t)=>{f(t),i({...s,approver_user_ids:JSON.stringify(t.map(c=>c.email))})},loading:G,filterOptions:(a,{inputValue:t})=>a.filter(c=>c.email.toLowerCase().includes(t.toLowerCase())||(c.username||"").toLowerCase().includes(t.toLowerCase())),renderInput:a=>e.jsx(d,{...a,label:l("settings.approvalConfig.approverConfig.approverUserIds"),placeholder:"搜索用户邮箱",helperText:l("settings.approvalConfig.approverConfig.approverUserIdsHelper")}),renderTags:(a,t)=>a.map((c,h)=>n.createElement(b,{...t({index:h}),key:c.email,label:c.email}))})})})]}),e.jsxs(j,{defaultExpanded:!0,children:[e.jsx(y,{expandIcon:e.jsx(x,{}),children:e.jsx(o,{variant:"subtitle1",fontWeight:"600",children:l("settings.approvalConfig.formFields")})}),e.jsx(w,{children:e.jsxs(_,{spacing:2,children:[e.jsx(A,{severity:"info",children:l("settings.approvalConfig.formFieldsHelper")}),e.jsx(d,{fullWidth:!0,label:l("settings.approvalConfig.formFieldsJson"),value:s.form_fields,onChange:a=>i({...s,form_fields:a.target.value}),multiline:!0,rows:12,required:!0,sx:{fontFamily:"monospace","& textarea":{fontFamily:"monospace",fontSize:"0.875rem"}}}),e.jsxs(j,{children:[e.jsx(y,{expandIcon:e.jsx(x,{}),children:e.jsx(o,{variant:"body2",color:"primary",children:l("settings.approvalConfig.viewExample")})}),e.jsx(w,{children:e.jsx(p,{sx:{bgcolor:"#f5f5f5",p:2,borderRadius:1,fontFamily:"monospace",fontSize:"0.75rem",overflow:"auto"},children:e.jsxs("pre",{style:{margin:0},children:[s.type==="feishu"&&u.feishu,s.type==="dingtalk"&&u.dingtalk,s.type==="wechat"&&u.wechat]})})})]})]})})]}),e.jsx(fe,{control:e.jsx(ye,{checked:s.enabled,onChange:a=>i({...s,enabled:a.target.checked})}),label:l("settings.approvalConfig.enableConfig")})]})}),e.jsxs(he,{children:[e.jsx(v,{onClick:I,children:l("common.cancel")}),e.jsx(v,{variant:"contained",onClick:X,disabled:!s.name||!s.app_id||!s.app_secret,children:l(g?"common.save":"common.create")})]})]})]})})}export{Ue as default};
