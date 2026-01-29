import{d as _,L as f,z as m,u as y,r as c,a0 as l,g as E,a4 as j,a5 as S,a6 as T,a7 as d,S as H,o as A,a2 as I,h as u,i as p,k as R,j as v,F as b,_ as L}from"./index-35337fd3.js";const k={key:0,style:{overflow:"hidden"}},C=["src","height"],O=_({__name:"index",setup(G){const w=f(),i=m(),h=y(),t=c(""),o=c(700);w.beforeEach((e,a)=>{console.log("router.beforeEach",e,a),e.fullPath=="/preview"&&r()}),i.query?.language&&(i.query?.language.toUpperCase()=="EN"?document.title="Scada Home":document.title="\u7EC4\u6001\u4E3B\u9875");const r=async()=>{let e=h?.projectDetail?.id?.toString();l.type==="gateway"&&E("deviceType")!="phone"&&(e="1");let a=await j({projectId:e});if(a.code==200&&a.data!=null){const n=S(T.CHART_PREVIEW_NAME,"href");if(!n)return;const g=`${window.location.protocol}//${window.location.host}`;t.value=g+n+"/"+a.data.id}else t.value="",d(H.GO_CHART_STORAGE_LIST),d("GO_CHART_STORAGE_LIST_TOW")},s=()=>{l.type=="gateway"?o.value=window.innerHeight:o.value=window.innerHeight-100};return A(()=>{r(),s(),window.addEventListener("resize",s)}),I(()=>{window.removeEventListener("resize",s)}),(e,a)=>(u(),p(b,null,[t.value?(u(),p("div",k,[R("iframe",{src:t.value,frameborder:"0",width:"100%",height:o.value},null,8,C)])):v("v-if",!0),v(` <div class="go-project-my-template"
       v-else>
    <n-space vertical>
      <n-image object-fit="contain"
               height="300"
               preview-disabled
               :src="requireErrorImg()"></n-image>
      <n-h3>\u6682\u65F6\u8FD8\u6CA1\u6709\u4E1C\u897F\u5462</n-h3>
    </n-space>
  </div> `)],2112))}}),x=L(O,[["__scopeId","data-v-387f01ec"],["__file","/Users/cyj/Desktop/work project/Scada-4.21/newScada/src/views/project/projectHome/index.vue"]]);export{x as default};
