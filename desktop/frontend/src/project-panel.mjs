import './project-panel.css';
import {CreateProject,StartPortTunnel,PortTunnels} from '../wailsjs/go/main/App.js';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime.js';
import {projectTemplates,validProjectName,vscodeProjectURL} from './project-model.mjs';
import {icon} from './ui-icons.mjs';

export function initProjectPanel({getSession,getName,onCreated}){
  const dialog=document.createElement('dialog');dialog.id='projectDialog';dialog.setAttribute('aria-labelledby','projectTitle');
  dialog.innerHTML=`<form id="projectForm" novalidate><div class="project-header"><div>${icon('folder')}<span><h2 id="projectTitle">新建项目</h2><p id="projectServer"></p></span></div><button type="button" data-project-close aria-label="关闭">×</button></div>
    <div class="project-body"><label>所选目录<input name="parent" readonly aria-label="所选目录"><small>项目将在这个目录中创建。可关闭窗口，重新选择文件夹。</small></label><label>项目名称<input name="projectName" placeholder="my-project" maxlength="64" autocomplete="off"><small>以字母开头，可用字母、数字、短横线和下划线。</small></label><div class="project-template-label">选择起始模板</div><div class="project-template-grid">${projectTemplates.map(t=>`<button type="button" data-project-template="${t.id}" aria-pressed="${t.id==='empty'}"><b>${t.label}</b><small>${t.hint}</small></button>`).join('')}</div><div id="projectTemplateInfo"></div><div class="project-location-preview"><span>项目路径</span><code id="projectLocationPreview"></code></div>
    <section class="project-vscode"><b>VS Code · 服务器端口 10020</b><small>直接连接 127.0.0.1:10020，自动建立或复用 SSH 本地转发，无需选择端口或预先配置 Listen。远端项目路径须在 VS Code 中可见。</small></section>
    <p class="project-note">创建后在浏览器的 VS Code 中打开项目目录。NekSSH 文件列表仍使用内置编辑器。不安装依赖、不运行程序、不覆盖已有目录。</p><p id="projectMessage" role="status" hidden></p></div>
    <div class="project-footer"><button type="button" data-project-close>取消</button><button type="submit" class="primary" id="projectSubmit">创建并进入项目</button></div></form>`;
  document.body.append(dialog);
  const $=s=>dialog.querySelector(s),form=$('form'),field=name=>form.elements.namedItem(name);
  let session='',revision=0,submitting=false,createdPath='',template='empty';
  const valid=(id,version)=>dialog.open&&session===id&&getSession()===id&&revision===version;
  function message(text,error=false){$('#projectMessage').textContent=text;$('#projectMessage').hidden=!text;$('#projectMessage').classList.toggle('error',error)}
  function update(){
    const selected=projectTemplates.find(t=>t.id===template);
    $('#projectTemplateInfo').textContent=selected.files.length?'将创建：'+selected.files.join(' · '):'只创建空目录，不生成文件。';
    $('#projectLocationPreview').textContent=field('parent').value.replace(/\/$/,'')+'/'+(field('projectName').value.trim()||'项目名称');
    $('#projectSubmit').disabled=submitting||(!createdPath&&!validProjectName(field('projectName').value.trim()));
    $('#projectSubmit').textContent=submitting?'处理中…':createdPath?'重试接入 VS Code':'创建并接入 VS Code';
    field('projectName').disabled=submitting||!!createdPath;
    dialog.querySelectorAll('[data-project-template]').forEach(button=>{button.setAttribute('aria-pressed',String(button.dataset.projectTemplate===template));button.disabled=submitting||!!createdPath});
    dialog.querySelectorAll('[data-project-close]').forEach(button=>button.disabled=submitting);
  }
  function close(){if(submitting)return;revision++;dialog.close()}
  function open(path){
    const id=getSession();if(!id||submitting)return;
    revision++;session=id;createdPath='';template='empty';form.reset();
    $('#projectServer').textContent=getName();field('parent').value=path;message('');update();dialog.showModal();field('projectName').focus();
  }
  form.addEventListener('input',update);
  form.addEventListener('change',update);
  dialog.querySelectorAll('[data-project-template]').forEach(button=>button.onclick=()=>{template=button.dataset.projectTemplate;update()});
  dialog.querySelectorAll('[data-project-close]').forEach(button=>button.onclick=close);
  dialog.addEventListener('cancel',event=>{event.preventDefault();close()});
  form.onsubmit=async event=>{
    event.preventDefault();if(submitting)return;
    const id=session,version=revision;
    if(!valid(id,version)){message('SSH 会话已切换或断开，请关闭后重新选择目录。',true);return}
    const name=field('projectName').value.trim(),parent=field('parent').value;
    if(!createdPath&&(!validProjectName(name)||!parent)){message('请输入有效项目名称。',true);return}
    submitting=true;message('');update();
    try{
      const selected=await StartPortTunnel(id,'tcp','127.0.0.1',10020);
      vscodeProjectURL(selected.LocalAddress,'/');
      const checkTunnel=async()=>{
        const active=await PortTunnels(id);
        if(!valid(id,version))throw Error('SSH 会话已切换或断开，请回到原服务器后重试。');
        if(!active?.some(t=>t.ID===selected.ID&&t.LocalAddress===selected.LocalAddress&&t.RemoteAddress===selected.RemoteAddress))throw Error('所选 Listen 转发已停止或改变，请刷新后重新选择。');
      };
      await checkTunnel();
      if(!createdPath){const result=await CreateProject(id,parent,name,template);createdPath=result.Path}
      if(!valid(id,version))throw Error(`项目已创建在原服务器：${createdPath}。当前 SSH 会话已切换或断开，请回到原服务器查看。`);
      await onCreated(createdPath);
      await checkTunnel();
      await BrowserOpenURL(vscodeProjectURL(selected.LocalAddress,createdPath));
      submitting=false;close();
    }catch(error){message((createdPath?`项目已创建：${createdPath}。无需重复创建，可重试接入。\n`:'')+String(error),true)}
    finally{submitting=false;update()}
  };
  return {create:open};
}
