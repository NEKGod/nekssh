import './style.css';
import './terminal.css';
import './hosts.css';
import '@xterm/xterm/css/xterm.css';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon } from '@xterm/addon-search';
import { basicSetup } from 'codemirror';
import { EditorView, keymap } from '@codemirror/view';
import { EditorState } from '@codemirror/state';
import { javascript } from '@codemirror/lang-javascript';
import { json } from '@codemirror/lang-json';
import { html } from '@codemirror/lang-html';
import { css } from '@codemirror/lang-css';
import { python } from '@codemirror/lang-python';
import { go } from '@codemirror/lang-go';
import { java } from '@codemirror/lang-java';
import { php } from '@codemirror/lang-php';
import { rust } from '@codemirror/lang-rust';
import { sql } from '@codemirror/lang-sql';
import { xml } from '@codemirror/lang-xml';
import { markdown } from '@codemirror/lang-markdown';
import { HighlightStyle, StreamLanguage, syntaxHighlighting } from '@codemirror/language';
import { tags } from '@lezer/highlight';
import { shell } from '@codemirror/legacy-modes/mode/shell';
import { yaml } from '@codemirror/legacy-modes/mode/yaml';
import { diffLines } from 'diff';
import 'highlight.js/styles/github-dark.css';
import { collectMarkdownOutline, renderMarkdown, renderMermaidBlocks } from './markdown-render.js';
import { CommandHistory, Connect, CreateSystemdService, DeletePath, Disconnect, DownloadDirectory, DownloadFile, FileHistory, Input, IsELF, ListFiles, MakeDirectory, OwnershipOptions, ProtectSecret, ReadFileHistory, ReadRemoteFile, ReadRemoteImage, ReadRemotePDF, RecordFileHistory, Resize, SaveRemoteFile, ScanConnectionHostKey, SelectPrivateKeyFile, ServerPartitions, ServerProcesses, ServerStats, SetPathMode, SetPathOwner, ShowInFolder, SignalProcess, SystemdDaemonReload, SystemdServiceAction, SystemdServicePath, SystemdServices, SetDebug, TrustHostKey, UnprotectSecret, UploadFile, UploadLocalFile } from '../wailsjs/go/main/App.js';
import { ClipboardGetText, ClipboardSetText, EventsOn, OnFileDrop } from '../wailsjs/runtime/runtime.js';

document.querySelector('#app').innerHTML = `
<div class="shell">
  <aside>
    <h1>NekSSH</h1>
    <div class="side-panel" id="serverPanel">
      <div class="panel-title"><span>资源列表</span><small class="server-name">未连接</small></div>
      <div class="resource" data-level="idle">
        <div class="resource-head"><span>CPU</span><b id="cpuValue">--</b></div>
        <div class="resource-bar"><i id="cpuBar"></i></div>
      </div>
      <div class="resource" data-level="idle">
        <div class="resource-head"><span>内存</span><b id="memValue">--</b></div>
        <div class="resource-bar"><i id="memBar"></i></div>
      </div>
      <div class="resource" data-level="idle">
        <div class="resource-head"><span>硬盘</span><b id="diskValue">--</b></div>
        <div class="resource-bar"><i id="diskBar"></i></div>
      </div>
      <div class="resource" data-level="idle">
        <div class="resource-head"><span>虚拟内存</span><b id="swapValue">--</b></div>
        <div class="resource-bar"><i id="swapBar"></i></div>
      </div>
    </div>
    <div class="side-panel" id="processPanel">
      <div class="panel-title"><span>进程列表</span><small>CPU 排序</small></div>
      <div class="process-search"><input id="processSearch" placeholder="搜索进程、PID 或端口"></div>
      <div class="process-head"><button data-process-sort="PID">PID</button><button data-process-sort="CPU">CPU</button><button data-process-sort="Memory">内存</button><button data-process-sort="Command">进程</button></div>
      <div id="processList"><div class="panel-empty">连接后显示进程</div></div>
    </div>
    <div class="side-panel" id="partitionPanel">
      <div class="panel-title"><span>分区列表</span><small>磁盘空间</small></div>
      <div id="partitionList"><div class="panel-empty">连接后显示分区</div></div>
    </div>
  </aside>
  <main>
    <header><div class="top-left"><button id="hostMenuButton">连接列表</button><button id="transferButton">传输列表 <b id="transferBadge" hidden>0</b></button><button id="commandHistoryButton">命令历史</button><span id="sessionTitle">未连接</span><span id="reflectInfo" hidden></span></div><button id="debugButton" type="button">调试日志</button><span id="status">● 就绪</span></header>
    <div id="sessionTabs"><div class="tabs-empty">暂无连接</div></div>
    <div id="hostMenu" hidden>
      <div class="host-menu-head"><b>连接列表</b><button id="newConnection" class="primary">新建连接</button></div>
      <div id="hosts"></div>
    </div>
    <div id="transferPanel" hidden><div class="transfer-head"><b>文件传输</b><button id="clearTransfers">清除已完成</button></div><div class="transfer-tabs"><button data-transfer-tab="upload" class="active">上传列表</button><button data-transfer-tab="download">下载列表</button></div><div id="transferList"></div></div>
    <div id="commandHistoryPanel" hidden><div class="command-history-head"><b>命令历史</b><button id="clearCommandHistory">刷新</button></div><div class="command-history-search"><input id="commandHistorySearch" placeholder="搜索当前连接的命令"></div><div id="commandHistoryList"></div></div>
    <section id="welcome"><h2>连接 SSH 服务器</h2><p>连接在本机桌面客户端内建立。</p></section>
    <section id="workspace" hidden>
      <div id="terminal"></div>
      <div id="files">
        <div class="filebar">
          <div class="file-location"><button id="upDirectory" class="icon-button" title="返回上级">↑</button><button id="pathToggle" class="icon-button" title="输入目录跳转">⌘</button><div id="breadcrumbs"></div><dialog class="path-jump"><div class="path-jump-head"><div><b>跳转到目录</b><span id="pathCurrent">当前目录：.</span></div><button id="pathJumpClose" class="icon-button" title="关闭">×</button></div><div class="path-jump-body"><div class="path-input-wrap"><span>/</span><input id="pathInput" placeholder="例如 /var/www/html"></div><button id="pathGo" class="primary">进入目录</button></div></dialog></div>
          <div class="file-actions"><span id="fileCount">0 项</span><button id="mkdir">＋ 新建目录</button><button id="upload">↑ 上传</button><button id="refresh" class="icon-button" title="刷新">↻</button></div>
        </div>
        <div class="bookmarkbar"><button id="saveBookmark">☆ 保存当前目录</button><button id="bookmarkButton">快捷目录 <span>▾</span></button><dialog id="bookmarkMenu"></dialog></div>
        <div id="fileList" tabindex="0"></div>
      </div>
    </section>
  </main>
</div>
<dialog id="editorDialog"><div class="editor-window">
  <div class="editor-titlebar"><div><b id="editorFileName">代码编辑器</b><span id="editorPath"></span></div><div class="editor-commands"><span id="editorState">已保存</span><button id="editorRefresh">从服务器刷新</button><button id="editorHistory">历史记录</button><button id="editorSave" class="primary">保存</button><button id="editorClose" class="icon-button" title="关闭">×</button></div></div>
  <div class="editor-body"><div id="codeEditor"></div><aside id="historyPanel" hidden><div class="history-title"><b>编辑历史</b><button id="historyClose" class="icon-button">×</button></div><div id="historyList"></div></aside><div id="diffPanel" hidden><div class="diff-title"><b>版本对比</b><span>绿色为新增，红色为删除</span><button id="diffClose" class="icon-button">×</button></div><pre id="diffContent"></pre></div></div>
  <div class="editor-status"><span id="editorLanguage">纯文本</span><span>Ctrl+S 保存 · Ctrl+F 查找</span></div>
</div></dialog>
<dialog id="connectDialog"><form id="connectForm"><h2 id="dialogTitle">新建 SSH 连接</h2>
  <label>名称<input name="name" required placeholder="生产服务器"></label>
  <label>主机<input name="host" required placeholder="192.168.1.10"></label>
  <div class="row"><label>用户名<input name="username" required placeholder="root"></label><label class="port">端口<input name="port" type="number" value="22"></label></div>
  <label>密码<input name="password" type="password" autocomplete="current-password"></label>
  <label class="remember-password"><input name="rememberPassword" type="checkbox"><span>记住密码（Windows 加密存储）</span></label>
  <label>私钥文件<div class="key-file-row"><input name="privateKeyPath" readonly placeholder="未选择私钥文件"><button type="button" id="selectPrivateKey">选择文件</button><button type="button" id="clearPrivateKey">清除</button></div><textarea name="privateKey" hidden></textarea></label>
  <details class="connection-options"><summary>代理服务器</summary><div class="option-body"><label>代理类型<select name="proxyType"><option value="direct">不使用代理</option><option value="socks5">SOCKS5</option><option value="http">HTTP CONNECT</option></select></label><div class="row"><label>代理地址<input name="proxyHost" placeholder="127.0.0.1"></label><label class="port">端口<input name="proxyPort" type="number" value="1080"></label></div><div class="row"><label>代理用户名<input name="proxyUsername"></label><label>代理密码<input name="proxyPassword" type="password"></label></div></div></details>
  <details class="connection-options"><summary>Listen 本地转发</summary><div class="option-body"><div class="forward-toolbar"><span>转发规则</span><button type="button" id="addForward">＋ 添加规则</button></div><div id="forwardList"><div class="forward-empty">暂无转发规则</div></div></div></details>
  <div class="actions"><button type="button" id="cancel">取消</button><button type="submit" class="primary">保存并连接</button></div>
</form></dialog>
<dialog id="permissionDialog"><div class="permission-window"><div class="permission-head"><div><b>文件属性</b><span id="permissionPath"></span></div><button id="permissionClose" class="icon-button">×</button></div><div class="permission-body"><label>权限<select id="permissionMode"><option value="644">644 · 标准文件</option><option value="600">600 · 私密文件</option><option value="664">664 · 组可写文件</option><option value="755">755 · 标准目录/程序</option><option value="700">700 · 私有目录/程序</option><option value="775">775 · 组可写目录</option><option value="777">777 · 所有人可写</option></select></label><div class="owner-fields"><label>所有者<select id="permissionOwner"></select></label><label>用户组<select id="permissionGroup"></select></label></div><div class="permission-preview"><span>所有者权限</span><span>用户组权限</span><span>其他用户权限</span></div></div><div class="permission-actions"><button id="permissionCancel">取消</button><button id="permissionSave" class="primary">应用属性</button></div></div></dialog>
<dialog id="forwardDialog"><div class="forward-window"><div class="permission-head"><div><b id="forwardDialogTitle">添加转发规则</b><span>本地端口通过当前 SSH 会话转发</span></div><button id="forwardClose" class="icon-button">×</button></div><div class="forward-body"><label>规则名称<input id="forwardName" placeholder="Web 服务"></label><label class="remember-password"><input id="forwardEnabled" type="checkbox" checked><span>启用此规则</span></label><div class="row"><label>监听地址<input id="forwardListenAddress" value="127.0.0.1"></label><label class="port">监听端口<input id="forwardListenPort" type="number" placeholder="8080"></label></div><div class="row"><label>目标地址<input id="forwardTargetHost" value="127.0.0.1"></label><label class="port">目标端口<input id="forwardTargetPort" type="number" placeholder="80"></label></div></div><div class="permission-actions"><button id="forwardCancel">取消</button><button id="forwardSave" class="primary">保存规则</button></div></div></dialog>
<dialog id="imageDialog"><div class="image-viewer"><div class="image-toolbar"><div><b id="imageName">图片查看器</b><span id="imageInfo"></span></div><div><button id="imageZoomOut" class="icon-button" title="缩小">−</button><button id="imageFit">适应窗口</button><button id="imageActual">原始大小</button><button id="imageZoomIn" class="icon-button" title="放大">＋</button><button id="imageDownload">下载</button><button id="imageClose" class="icon-button" title="关闭">×</button></div></div><div id="imageCanvas"><img id="imagePreview"></div></div></dialog>
<dialog id="markdownDialog"><div class="markdown-viewer"><div class="image-toolbar"><div><b id="markdownName">Markdown</b><span id="markdownPath"></span></div><div><button id="markdownEdit">代码编辑</button><button id="markdownRefresh">刷新</button><button id="markdownClose" class="icon-button" title="关闭">×</button></div></div><div id="markdownLayout" class="markdown-layout"><aside id="markdownToc" hidden></aside><article id="markdownBody" class="markdown-body"></article></div><div id="flowOverlay" hidden><div class="image-toolbar"><div><b>流程图</b><span id="flowZoomLabel">100%</span></div><div><button id="flowZoomOut" class="icon-button" title="缩小">−</button><button id="flowFit">适应窗口</button><button id="flowZoomIn" class="icon-button" title="放大">＋</button><button id="flowClose" class="icon-button" title="关闭">×</button></div></div><div id="flowViewport"><div id="flowStage"></div></div></div></div></dialog>
<dialog id="pdfDialog"><div class="image-viewer"><div class="image-toolbar"><div><b id="pdfName">PDF</b><span id="pdfInfo"></span></div><div><button id="pdfDownload">下载</button><button id="pdfClose" class="icon-button" title="关闭">×</button></div></div><iframe id="pdfFrame" title="PDF 预览"></iframe></div></dialog>
<dialog id="createServiceDialog"><div class="service-create-window"><div class="permission-head"><div><b>创建 systemd 服务</b><span id="serviceExecutable"></span></div><button id="createServiceClose" class="icon-button">×</button></div><div class="service-create-body"><label>服务名称<input id="serviceName" placeholder="my-app"></label><label>描述<input id="serviceDescription" placeholder="My Application"></label><label>启动参数<input id="serviceArguments" placeholder="--config /etc/app.conf"></label><div class="row"><label>运行用户<input id="serviceUser" placeholder="root"></label><label>工作目录<input id="serviceWorkingDirectory"></label></div><label class="remember-password"><input id="serviceEnableNow" type="checkbox" checked><span>创建后设为开机启动并立即运行</span></label></div><div class="permission-actions"><button id="createServiceCancel">取消</button><button id="createServiceSave" class="primary">创建服务</button></div></div></dialog>
<dialog id="serviceManagerDialog"><div class="service-manager-window"><div class="service-manager-head"><b>systemd 服务</b><input id="serviceSearch" placeholder="搜索服务"><button id="serviceRefresh">刷新</button><button id="serviceManagerClose" class="icon-button">×</button></div><div id="serviceList"></div></div></dialog>
<div id="fileContextMenu" hidden></div><div id="tabContextMenu" hidden></div><div id="processContextMenu" hidden></div><div id="terminalContextMenu" hidden></div>`;

const $ = selector => document.querySelector(selector);
const fit = new FitAddon();
const search = new SearchAddon();
const term = new Terminal({
  cursorBlink:true,
  scrollback:10000,
  fontSize:14,
  fontFamily:'Consolas, Cascadia Mono, monospace',
  theme:{background:'#080b10',foreground:'#d8dee9',cursor:'#73daca'}
});

term.loadAddon(fit);
term.loadAddon(search);
term.open($('#terminal'));

let sessionId = '', currentPath = '.', editingIndex = -1;
const sshSessions=new Map();
let hosts = JSON.parse(localStorage.getItem('nekssh.desktop.hosts') || '[]');
let directoryBookmarks=JSON.parse(localStorage.getItem('nekssh.desktop.directoryBookmarks')||'[]');
let terminalMatchIndex=-1;
let terminalMatches=[];
let statsTimer=0;
let editorView=null, editorRemotePath='', editorDirty=false;
let editorSystemdConfig=false;
let permissionTarget=null;
let processItems=[],processSort={field:'CPU',direction:-1};
let editingForwards=[],forwardEditingIndex=-1;
let imagePreviewFile=null,imageZoom=1;
let serviceExecutableFile=null,systemdServiceItems=[];
let commandHistoryIndex=0,filteredCommandHistory=[];
let terminalCopyMode=false;
const transfers=new Map();
let transferTab='upload';
const editorHighlightStyle=HighlightStyle.define([
  {tag:tags.comment,color:'#66788f',fontStyle:'italic'},
  {tag:[tags.keyword,tags.modifier,tags.operatorKeyword],color:'#c792ea'},
  {tag:[tags.string,tags.special(tags.string)],color:'#a8d08d'},
  {tag:[tags.number,tags.bool,tags.null],color:'#f0b56a'},
  {tag:[tags.function(tags.variableName),tags.function(tags.propertyName)],color:'#75bfff'},
  {tag:[tags.typeName,tags.className,tags.namespace],color:'#e5c07b'},
  {tag:tags.variableName,color:'#d7dee9'},
  {tag:tags.propertyName,color:'#8bd5ca'},
  {tag:[tags.regexp,tags.escape],color:'#e06c75'},
  {tag:[tags.tagName,tags.heading],color:'#f07178'},
  {tag:[tags.attributeName,tags.labelName],color:'#ffcb6b'},
  {tag:[tags.punctuation,tags.bracket],color:'#9aa7b8'},
  {tag:[tags.invalid],color:'#ffffff',backgroundColor:'#c74b50'}
]);

function saveHosts(){
  localStorage.setItem('nekssh.desktop.hosts', JSON.stringify(hosts));
  renderHosts();
}

function renderHosts(){
  $('#hosts').innerHTML='';
  if(!hosts.length){
    $('#hosts').innerHTML='<div class="host-empty">暂无已保存的连接</div>';
    return;
  }
  hosts.forEach((host,index)=>{
    const row=document.createElement('div');
    row.className='saved-host';
    row.innerHTML=`<span><b>${escapeHtml(host.name)}</b><small>${escapeHtml(host.username)}@${escapeHtml(host.host)}:${host.port}</small></span><span><button data-edit="1">编辑</button><button data-delete="1">删除</button></span>`;
    row.onclick=event=>{
      if(event.target.dataset.delete){
        event.stopPropagation();
        if(confirm(`删除 ${host.name}？`)){hosts.splice(index,1);saveHosts()}
      }else if(event.target.dataset.edit){
        event.stopPropagation();
        $('#hostMenu').hidden=true;
        openDialog(host,index);
      }else{
        $('#hostMenu').hidden=true;
        if(host.passwordSecret||host.privateKeySecret) connectSavedHost(host);
        else openDialog(host,index);
      }
    };
    $('#hosts').append(row);
  });
}

async function openDialog(host={}, index=-1){
  editingIndex=index;
  const form=$('#connectForm');
  form.reset();
  form.name.value=host.name||'';
  form.host.value=host.host||'';
  form.username.value=host.username||'';
  form.port.value=host.port||22;
  form.privateKeyPath.value=host.privateKeyPath||'';
  form.rememberPassword.checked=!!host.passwordSecret;
  form.proxyType.value=host.proxyType||'direct';
  form.proxyHost.value=host.proxyHost||'';
  form.proxyPort.value=host.proxyPort||1080;
  form.proxyUsername.value=host.proxyUsername||'';
  editingForwards=Array.isArray(host.forwards)?host.forwards.map(item=>({...item})):[];
  if(!editingForwards.length&&host.listenEnabled&&host.listenPort) editingForwards.push({name:'本地转发',enabled:true,listenAddress:host.listenAddress||'127.0.0.1',listenPort:host.listenPort,targetHost:host.targetHost||'127.0.0.1',targetPort:host.targetPort});
  renderForwardList();
  try{
    form.password.value=host.passwordSecret?await UnprotectSecret(host.passwordSecret):'';
    form.privateKey.value=host.privateKeySecret?await UnprotectSecret(host.privateKeySecret):'';
    form.proxyPassword.value=host.proxyPasswordSecret?await UnprotectSecret(host.proxyPasswordSecret):'';
  }catch(error){
    form.password.value='';
    form.privateKey.value='';
    form.proxyPassword.value='';
    alert(`无法读取已保存的连接凭据：${error}`);
  }
  $('#dialogTitle').textContent=index<0?'新建 SSH 连接':`连接 ${host.name}`;
  $('#connectDialog').showModal();
  setTimeout(()=>form.password.focus(),50);
}

term.onData(data=>{
  if(!sessionId) return;
  trackTerminalInput(sessionId,data);
  Input(sessionId,data);
});
term.onResize(({cols,rows})=>sessionId&&Resize(sessionId,cols,rows));
new ResizeObserver(()=>fit.fit()).observe($('#terminal'));

function setTerminalCopyMode(enabled){
  terminalCopyMode=enabled;
  term.options.mouseEventsRequireAlt=enabled;
  $('#terminal').classList.toggle('copy-mode',enabled);
  setStatus(enabled?'终端复制模式：拖动选择，Ctrl+C 复制；按住 Alt 操作终端鼠标，Esc 退出':'已退出终端复制模式',enabled?'#73daca':'#98a6b8');
  if(!enabled) term.focus();
}
$('#terminal').addEventListener('contextmenu',event=>{
  event.preventDefault();
  event.stopPropagation();
  const menu=$('#terminalContextMenu');
  menu.innerHTML=`<button data-terminal-action="copy" ${term.hasSelection()?'':'disabled'}>复制选中内容</button><button data-terminal-action="copyMode">${terminalCopyMode?'退出复制模式':'进入复制模式'}</button><button data-terminal-action="paste">粘贴</button><div class="context-separator"></div><button data-terminal-action="services">systemd 服务管理</button><button data-terminal-action="selectAll">全选终端内容</button><button data-terminal-action="clear">清空终端</button>`;
  menu.hidden=false;
  const width=menu.offsetWidth,height=menu.offsetHeight;
  menu.style.left=`${Math.max(8,Math.min(event.clientX,window.innerWidth-width-8))}px`;
  menu.style.top=`${Math.max(8,Math.min(event.clientY,window.innerHeight-height-8))}px`;
  menu.querySelectorAll('[data-terminal-action]').forEach(button=>button.onclick=async()=>{
    menu.hidden=true;
    const action=button.dataset.terminalAction;
    if(action==='copy'&&term.hasSelection()) await ClipboardSetText(term.getSelection());
    if(action==='copyMode') setTerminalCopyMode(!terminalCopyMode);
    if(action==='paste'){
      const targetSession=sessionId;
      const text=await ClipboardGetText();
      if(text&&targetSession&&targetSession===sessionId){trackTerminalInput(targetSession,text);await Input(targetSession,text)}
    }
    if(action==='selectAll') term.selectAll();
    if(action==='clear'){term.clear();const state=sshSessions.get(sessionId);if(state){state.outputChunks=[];state.outputBytes=0}}
    if(action==='services') openServiceManager();
    if(!terminalCopyMode) term.focus();
  });
});
document.addEventListener('pointerdown',event=>{if(!event.target.closest('#terminalContextMenu'))$('#terminalContextMenu').hidden=true});
document.addEventListener('keydown',event=>{if(event.key==='Escape')$('#terminalContextMenu').hidden=true});
window.addEventListener('blur',()=>{$('#terminalContextMenu').hidden=true});
$('#newConnection').onclick=()=>{$('#hostMenu').hidden=true;openDialog()};
$('#hostMenuButton').onclick=event=>{
  event.stopPropagation();
  const menu=$('#hostMenu');
  menu.hidden=!menu.hidden;
};
document.addEventListener('pointerdown',event=>{if(!event.target.closest('#hostMenu')&&!event.target.closest('#hostMenuButton'))$('#hostMenu').hidden=true});
$('#cancel').onclick=()=>$('#connectDialog').close();
$('#selectPrivateKey').onclick=async()=>{
  try{
    const selected=await SelectPrivateKeyFile();
    if(selected.Path){$('#connectForm').privateKeyPath.value=selected.Path;$('#connectForm').privateKey.value=selected.Content}
  }catch(error){alert(String(error))}
};
$('#clearPrivateKey').onclick=()=>{$('#connectForm').privateKeyPath.value='';$('#connectForm').privateKey.value=''};
function renderForwardList(){
  const list=$('#forwardList');
  if(!editingForwards.length){list.innerHTML='<div class="forward-empty">暂无转发规则</div>';return}
  list.innerHTML=editingForwards.map((rule,index)=>`<div class="forward-item ${rule.enabled?'':'disabled'}"><i></i><div><b>${escapeHtml(rule.name||'未命名规则')}</b><span>${escapeHtml(rule.listenAddress)}:${rule.listenPort} → ${escapeHtml(rule.targetHost)}:${rule.targetPort}</span></div><button type="button" data-forward-edit="${index}">编辑</button><button type="button" data-forward-delete="${index}">删除</button></div>`).join('');
  list.querySelectorAll('[data-forward-edit]').forEach(button=>button.onclick=()=>openForwardDialog(Number(button.dataset.forwardEdit)));
  list.querySelectorAll('[data-forward-delete]').forEach(button=>button.onclick=()=>{editingForwards.splice(Number(button.dataset.forwardDelete),1);renderForwardList()});
}

function openForwardDialog(index=-1){
  forwardEditingIndex=index;
  const rule=index>=0?editingForwards[index]:{name:'',enabled:true,listenAddress:'127.0.0.1',listenPort:'',targetHost:'127.0.0.1',targetPort:''};
  $('#forwardDialogTitle').textContent=index>=0?'编辑转发规则':'添加转发规则';
  $('#forwardName').value=rule.name||'';
  $('#forwardEnabled').checked=rule.enabled!==false;
  $('#forwardListenAddress').value=rule.listenAddress||'127.0.0.1';
  $('#forwardListenPort').value=rule.listenPort||'';
  $('#forwardTargetHost').value=rule.targetHost||'127.0.0.1';
  $('#forwardTargetPort').value=rule.targetPort||'';
  $('#forwardDialog').showModal();
  $('#forwardName').focus();
}

function saveForwardRule(){
  const rule={name:$('#forwardName').value.trim()||'本地转发',enabled:$('#forwardEnabled').checked,listenAddress:$('#forwardListenAddress').value.trim()||'127.0.0.1',listenPort:Number($('#forwardListenPort').value),targetHost:$('#forwardTargetHost').value.trim(),targetPort:Number($('#forwardTargetPort').value)};
  if(!rule.listenPort||!rule.targetHost||!rule.targetPort){alert('请完整填写监听端口和目标地址');return}
  const duplicate=editingForwards.some((item,index)=>index!==forwardEditingIndex&&item.listenAddress===rule.listenAddress&&Number(item.listenPort)===rule.listenPort);
  if(duplicate){alert('监听地址和端口不能重复');return}
  if(forwardEditingIndex>=0) editingForwards[forwardEditingIndex]=rule; else editingForwards.push(rule);
  $('#forwardDialog').close();
  renderForwardList();
}

$('#addForward').onclick=()=>openForwardDialog();
$('#forwardSave').onclick=saveForwardRule;
$('#forwardClose').onclick=$('#forwardCancel').onclick=()=>$('#forwardDialog').close();
$('#forwardDialog').addEventListener('cancel',event=>{event.preventDefault();$('#forwardDialog').close()});
initFileResize();
initSearchBox();
initWorkspaceShortcuts();

$('#connectForm').onsubmit=async event=>{
  event.preventDefault();
  const data=Object.fromEntries(new FormData(event.target));
  if(data.proxyType!=='direct'&&(!data.proxyHost.trim()||!Number(data.proxyPort))){alert('请填写代理服务器地址和端口');return}
  const saved={name:data.name.trim(),host:data.host.trim(),username:data.username.trim(),port:Number(data.port),privateKeyPath:data.privateKeyPath||'',proxyType:data.proxyType||'direct',proxyHost:data.proxyHost.trim(),proxyPort:Number(data.proxyPort)||0,proxyUsername:data.proxyUsername.trim(),forwards:editingForwards.map(item=>({...item}))};
  if(data.rememberPassword&&data.password) saved.passwordSecret=await ProtectSecret(data.password);
  if(data.privateKey) saved.privateKeySecret=await ProtectSecret(data.privateKey);
  if(data.proxyPassword) saved.proxyPasswordSecret=await ProtectSecret(data.proxyPassword);
  editingIndex<0?hosts.push(saved):hosts[editingIndex]=saved;
  saveHosts();
  await connectToHost(saved,data.password,data.privateKey,data.proxyPassword,true);
};

async function connectSavedHost(saved){
  setStatus('正在读取连接凭据','#e0af68');
  try{
    const password=saved.passwordSecret?await UnprotectSecret(saved.passwordSecret):'';
    const privateKey=saved.privateKeySecret?await UnprotectSecret(saved.privateKeySecret):'';
    const proxyPassword=saved.proxyPasswordSecret?await UnprotectSecret(saved.proxyPasswordSecret):'';
    await connectToHost(saved,password,privateKey,proxyPassword,false);
  }catch(error){
    setStatus('连接失败','#f7768e');
    alert(`无法使用已保存的连接凭据：${error}`);
  }
}

async function connectToHost(saved,password,privateKey,proxyPassword,closeDialog){
  const previousSessionId=sessionId;
  const newSessionId='session-'+Date.now()+'-'+Math.random().toString(16).slice(2);
  sshSessions.set(newSessionId,{saved,output:'',path:'.',connected:false,commandBuffer:'',commandHistory:[]});
  sessionId=newSessionId;
  renderSessionTabs();
  fit.fit();
  const request={ID:newSessionId,Host:saved.host,Username:saved.username,Password:password,PrivateKey:privateKey,Passphrase:'',Port:saved.port,Cols:term.cols,Rows:term.rows,ProxyType:saved.proxyType||'direct',ProxyHost:saved.proxyHost||'',ProxyPort:saved.proxyPort||0,ProxyUsername:saved.proxyUsername||'',ProxyPassword:proxyPassword||'',Forwards:(saved.forwards||[]).map(item=>({Name:item.name,Enabled:item.enabled,ListenAddress:item.listenAddress,ListenPort:item.listenPort,TargetHost:item.targetHost,TargetPort:item.targetPort}))};
  setStatus('正在连接','#e0af68');
  try{
    await connectWithTrust(request);
    if(closeDialog&&$('#connectDialog').open) $('#connectDialog').close();
    $('#welcome').hidden=true;
    $('#workspace').hidden=false;
    await activateSession(newSessionId);
  }catch(error){
    sshSessions.delete(newSessionId);
    if(sessionId===newSessionId) sessionId='';
    renderSessionTabs();
    setStatus('连接失败','#f7768e');
    alert(String(error));
    if(previousSessionId&&sshSessions.has(previousSessionId)) await activateSession(previousSessionId);
  }
}

async function activateSession(id){
  const state=sshSessions.get(id);
  if(!state) return;
  sessionId=id;
  currentPath=state.path||'.';
  $('#welcome').hidden=true;
  $('#workspace').hidden=false;
  $('#sessionTitle').textContent=`${state.saved.name} - ${state.saved.username}@${state.saved.host}`;
  setStatus(state.connected?'已连接':'已断开',state.connected?'#73daca':'#8996aa');
  term.reset();
  const replay=sessionOutputText(state);
  if(replay) term.write(replay);
  term.focus();
  fit.fit();
  renderSessionTabs();
  if(state.connected){
    showReflect(state.reflectAddress,state.reflectError);
    loadFiles(currentPath);
    startStatsRefresh(state.saved.name);
  }else{
    stopStatsRefresh();
    resetStatsPanel();
    showReflect('','');
  }
}

function renderSessionTabs(){
  const container=$('#sessionTabs');
  if(!sshSessions.size){
    container.innerHTML='<div class="tabs-empty">暂无连接</div>';
    return;
  }
  container.innerHTML=[...sshSessions.entries()].map(([id,state])=>`<button class="session-tab ${id===sessionId?'active':''}" data-session="${id}" title="${escapeHtml(state.saved.username)}@${escapeHtml(state.saved.host)}"><i class="${state.connected?'online':'offline'}"></i><span>${escapeHtml(state.saved.name)}</span><b data-close-session="${id}" title="关闭连接">×</b></button>`).join('');
  container.querySelectorAll('[data-session]').forEach(tab=>{
    tab.onclick=()=>activateSession(tab.dataset.session);
    tab.oncontextmenu=event=>openTabContextMenu(event,tab.dataset.session);
    tab.onauxclick=event=>{if(event.button===1){event.preventDefault();closeSessionTab(tab.dataset.session)}};
  });
  container.querySelectorAll('[data-close-session]').forEach(button=>button.onclick=event=>{event.stopPropagation();closeSessionTab(button.dataset.closeSession)});
}

function openTabContextMenu(event,targetID){
  event.preventDefault();
  event.stopPropagation();
  const ids=[...sshSessions.keys()];
  const index=ids.indexOf(targetID);
  const menu=$('#tabContextMenu');
  menu.innerHTML=`<div class="tab-menu-title">${escapeHtml(sshSessions.get(targetID)?.saved.name||'服务器')}</div><button data-action="close">关闭标签</button><button data-action="others" ${ids.length<2?'disabled':''}>关闭其他标签</button><button data-action="left" ${index<=0?'disabled':''}>关闭左侧标签</button><button data-action="right" ${index<0||index===ids.length-1?'disabled':''}>关闭右侧标签</button><div class="context-separator"></div><button data-action="all" class="danger">关闭全部标签</button>`;
  menu.hidden=false;
  const width=menu.offsetWidth,height=menu.offsetHeight;
  menu.style.left=`${Math.max(8,Math.min(event.clientX,window.innerWidth-width-8))}px`;
  menu.style.top=`${Math.max(8,Math.min(event.clientY,window.innerHeight-height-8))}px`;
  menu.querySelectorAll('[data-action]').forEach(button=>button.onclick=()=>{
    menu.hidden=true;
    const action=button.dataset.action;
    if(action==='close') closeSessionTabs([targetID]);
    if(action==='others') closeSessionTabs(ids.filter(id=>id!==targetID));
    if(action==='left') closeSessionTabs(ids.slice(0,index));
    if(action==='right') closeSessionTabs(ids.slice(index+1));
    if(action==='all') closeSessionTabs(ids);
  });
}

function closeSessionTabs(ids){
  const closing=new Set(ids);
  for(const id of closing){Disconnect(id);sshSessions.delete(id)}
  if(!sshSessions.has(sessionId)){
    const next=[...sshSessions.keys()].pop();
    if(next) activateSession(next);
    else showNoSession();
  }
  renderSessionTabs();
}

function showNoSession(){
  sessionId='';
  stopStatsRefresh();
  resetStatsPanel();
  term.reset();
  $('#workspace').hidden=true;
  $('#welcome').hidden=false;
  $('#sessionTitle').textContent='未连接';
  setStatus('就绪','#8996aa');
}

function closeSessionTab(id){
  const wasActive=id===sessionId;
  Disconnect(id);
  sshSessions.delete(id);
  if(wasActive){
    const next=[...sshSessions.keys()].pop();
    if(next) activateSession(next);
    else showNoSession();
  }
  renderSessionTabs();
}

function trackTerminalInput(id,data){
  const state=sshSessions.get(id);
  if(!state||!data) return;
  if(data.startsWith('\x1b')) return;
  for(const character of data){
    if(character==='\r'||character==='\n'){
      const command=state.commandBuffer.trim();
      if(command&&state.commandHistory[0]!==command) state.commandHistory.unshift(command);
      state.commandHistory=state.commandHistory.slice(0,200);
      state.commandBuffer='';
    }else if(character==='\x7f'||character==='\b'){
      state.commandBuffer=state.commandBuffer.slice(0,-1);
    }else if(character==='\x15'||character==='\x03'){
      state.commandBuffer='';
    }else if(character>=' '){
      state.commandBuffer+=character;
    }
  }
}

function renderCommandHistory(){
  const state=sshSessions.get(sessionId);
  const query=$('#commandHistorySearch').value.trim().toLowerCase();
  const commands=(state?.commandHistory||[]).filter(command=>!query||command.toLowerCase().includes(query));
  filteredCommandHistory=commands;
  commandHistoryIndex=Math.max(0,Math.min(commandHistoryIndex,commands.length-1));
  const list=$('#commandHistoryList');
  if(!commands.length){list.innerHTML='<div class="command-history-empty">暂无匹配命令</div>';return}
  list.innerHTML=commands.map((command,index)=>`<div class="command-history-item ${index===commandHistoryIndex?'active':''}" data-command-index="${index}"><code title="${escapeHtml(command)}">${escapeHtml(command)}</code><button data-command-input="${index}">输入</button><button data-command-run="${index}">执行</button></div>`).join('');
  list.querySelectorAll('[data-command-input]').forEach(button=>button.onclick=()=>sendHistoryCommand(commands[Number(button.dataset.commandInput)],false));
  list.querySelectorAll('[data-command-run]').forEach(button=>button.onclick=()=>sendHistoryCommand(commands[Number(button.dataset.commandRun)],true));
}

function sendHistoryCommand(command,execute){
  if(!sessionId||!command) return;
  trackTerminalInput(sessionId,execute?command+'\r':command);
  Input(sessionId,command+(execute?'\r':''));
  $('#commandHistoryPanel').hidden=true;
  term.focus();
}

async function openCommandHistory(){
  if(!sessionId) return;
  $('#commandHistoryPanel').hidden=false;
  $('#commandHistorySearch').value='';
  commandHistoryIndex=0;
  $('#commandHistoryList').innerHTML='<div class="command-history-empty">正在读取远程历史...</div>';
  const requestedSession=sessionId;
  try{
    const history=await CommandHistory(requestedSession);
    if(requestedSession!==sessionId) return;
    const state=sshSessions.get(requestedSession);
    if(state){
      const current=state.commandHistory||[];
      state.commandHistory=[...new Set([...current,...history])].slice(0,500);
    }
  }catch(error){console.warn(error)}
  renderCommandHistory();
  $('#commandHistorySearch').focus();
}

function closeCommandHistory(){
  $('#commandHistoryPanel').hidden=true;
  if(sessionId) term.focus();
}

$('#commandHistoryButton').onclick=event=>{event.stopPropagation();if($('#commandHistoryPanel').hidden)openCommandHistory();else closeCommandHistory()};
$('#commandHistorySearch').addEventListener('input',()=>{commandHistoryIndex=0;renderCommandHistory()});
$('#commandHistorySearch').addEventListener('keydown',event=>{
  if(!filteredCommandHistory.length) return;
  if(event.key==='ArrowDown'||event.key==='ArrowUp'){
    event.preventDefault();
    commandHistoryIndex=(commandHistoryIndex+(event.key==='ArrowDown'?1:-1)+filteredCommandHistory.length)%filteredCommandHistory.length;
    renderCommandHistory();
    $(`[data-command-index="${commandHistoryIndex}"]`)?.scrollIntoView({block:'nearest'});
  }else if(event.key==='Enter'){
    event.preventDefault();
    sendHistoryCommand(filteredCommandHistory[commandHistoryIndex],true);
  }
});
$('#clearCommandHistory').onclick=openCommandHistory;
document.addEventListener('pointerdown',event=>{if(!event.target.closest('#commandHistoryPanel')&&!event.target.closest('#commandHistoryButton')&&!$('#commandHistoryPanel').hidden)closeCommandHistory()});
document.addEventListener('keydown',event=>{if(event.key==='Escape'&&!$('#commandHistoryPanel').hidden){event.preventDefault();event.stopPropagation();closeCommandHistory()}},true);

async function connectWithTrust(request){
  try{
    return await Connect(request);
  }catch(error){
    if(!String(error).includes('主机指纹尚未信任')) throw error;
    const fingerprint=await ScanConnectionHostKey(request);
    if(!confirm(`首次连接此服务器。\n\nSHA-256 指纹：\n${fingerprint}\n\n确认信任并继续连接吗？`)) throw new Error('用户取消了主机指纹确认');
    await TrustHostKey(request.Host,request.Port);
    return Connect(request);
  }
}

function appendSessionOutput(state,data){
  if(!state.outputChunks) state.outputChunks=[];
  state.outputChunks.push(data);
  state.outputBytes=(state.outputBytes||0)+data.length;
  while(state.outputBytes>262144&&state.outputChunks.length>1){
    state.outputBytes-=state.outputChunks[0].length;
    state.outputChunks.shift();
  }
}
function sessionOutputText(state){
  return (state.outputChunks||[]).join('');
}
let terminalWriteFrame=0;
const terminalWrites=new Map();
function queueTerminalWrite(id,data){
  terminalWrites.set(id,(terminalWrites.get(id)||'')+data);
  if(terminalWriteFrame) return;
  terminalWriteFrame=requestAnimationFrame(()=>{
    terminalWriteFrame=0;
    const chunk=terminalWrites.get(sessionId)||'';
    terminalWrites.clear();
    if(chunk) term.write(chunk);
  });
}
EventsOn('ssh:output',(id,data)=>{
  const state=sshSessions.get(id);
  if(!state) return;
  appendSessionOutput(state,data);
  if(id===sessionId) queueTerminalWrite(id,data);
});
EventsOn('ssh:connected',id=>{
  const state=sshSessions.get(id);
  if(state) state.connected=true;
  renderSessionTabs();
  if(id===sessionId)setStatus(state?.forwardNote||'已连接',state?.forwardNote?'#e0af68':'#73daca');
});
EventsOn('forward:notice',(id,message)=>{
  const state=sshSessions.get(id);
  if(state) state.forwardNote=message;
  if(id===sessionId)setStatus(message,'#e0af68');
});
function showReflect(address,error){
  const info=$('#reflectInfo');
  if(address){
    info.hidden=false;
    info.textContent=`反射 http://${address}`;
    info.title='服务器上的程序访问这个地址即可调用本机。写入文件 POST /v1/files，运行并反射端口 POST /v1/run';
    return;
  }
  if(error){
    info.hidden=false;
    info.textContent='反射未建立';
    info.title=error;
    return;
  }
  info.hidden=true;
  info.textContent='';
}
EventsOn('reflect:ready',(id,address)=>{
  const state=sshSessions.get(id);
  if(state){state.reflectAddress=address;state.reflectError=''}
  if(id===sessionId) showReflect(address,'');
});
EventsOn('reflect:notice',(id,message)=>{
  const state=sshSessions.get(id);
  if(state) state.reflectError=message;
  if(id===sessionId) showReflect('',message);
});
EventsOn('ssh:closed',id=>{
  const state=sshSessions.get(id);
  if(state) state.connected=false;
  renderSessionTabs();
  if(id===sessionId){setStatus('已断开','#8996aa');stopStatsRefresh();resetStatsPanel()}
});
EventsOn('transfer:progress',progress=>{
  transfers.set(progress.ID,{...progress});
  renderTransfers();
});

let fileListToken=0;

async function loadFiles(remotePath){
  const requestedSession=sessionId;
  const token=++fileListToken;
  try{
    const entries=await ListFiles(requestedSession,remotePath);
    if(requestedSession!==sessionId||token!==fileListToken) return;
    currentPath=entries[0]?.Path?.replace(/\/[^/]+$/,'')||remotePath;
    const state=sshSessions.get(requestedSession);
    if(state) state.path=currentPath;
    renderBreadcrumbs(currentPath);
    $('#pathInput').value=currentPath;
    $('#pathCurrent').textContent=`当前目录：${currentPath}`;
    const list=$('#fileList');
    $('#fileCount').textContent=`${entries.length} 项`;
    list.innerHTML='<div class="file head"><span>名称</span><span>大小</span><span>修改时间</span><span>权限</span><span>所有者</span></div>';
    const parentEntry={Name:'..',Path:parentRemotePath(currentPath),Directory:true,Parent:true,Size:0,Modified:'',Mode:'',Owner:'',Group:''};
    for(const file of [parentEntry,...entries]){
      const row=document.createElement('div');
      row.className=`file ${file.Directory?'directory':'regular-file'} ${file.Parent?'parent-directory':''}`;
      row.dataset.search=(file.Name+' '+file.Path).toLowerCase();
      row._file=file;
      const type=file.Parent?'UP':file.Directory?'DIR':fileType(file.Name);
      row.innerHTML=`<span class="file-name"><i>${type}</i><b>${escapeHtml(file.Name)}</b></span><span>${file.Directory?'--':formatSize(file.Size)}</span><span>${formatFileTime(file.Modified)}</span><span class="file-mode">${escapeHtml(file.Mode)}</span><span>${file.Parent?'返回上级':`${escapeHtml(file.Owner)}:${escapeHtml(file.Group)}`}</span>`;
      row.onclick=()=>{
        list.querySelectorAll('.file.selected').forEach(item=>item.classList.remove('selected'));
        row.classList.add('selected');
        list.focus({preventScroll:true});
      };
      row.ondblclick=()=>openFileEntry(file);
      if(!file.Parent) row.oncontextmenu=event=>fileMenu(event,file);
      list.append(row);
    }
    if(!entries.length) list.insertAdjacentHTML('beforeend','<div class="file-empty"><b>此目录为空</b><span>可以上传文件或新建目录</span></div>');
  }catch(error){
    if(requestedSession===sessionId&&token===fileListToken) alert(String(error));
  }
}

function parentRemotePath(remotePath){
  if(!remotePath||remotePath==='.') return '..';
  if(remotePath==='/') return '/';
  const trimmed=remotePath.replace(/\/+$/,'');
  if(trimmed.startsWith('/')) return trimmed.replace(/\/[^/]*$/,'')||'/';
  return trimmed.replace(/\/[^/]*$/,'')||'.';
}

function scrollFileRowIntoView(row){
  if(!row) return;
  const container=$('#files');
  const containerRect=container.getBoundingClientRect();
  const rowRect=row.getBoundingClientRect();
  const visibleTop=containerRect.top+106;
  const visibleBottom=containerRect.bottom-4;
  if(rowRect.top<visibleTop) container.scrollTop-=visibleTop-rowRect.top;
  else if(rowRect.bottom>visibleBottom) container.scrollTop+=rowRect.bottom-visibleBottom;
}

function renderBreadcrumbs(remotePath){
  const container=$('#breadcrumbs');
  const absolute=remotePath.startsWith('/');
  const parts=remotePath.split('/').filter(Boolean);
  const crumbs=[];
  if(absolute) crumbs.push({label:'根目录',path:'/'});
  parts.forEach((part,index)=>crumbs.push({label:part,path:(absolute?'/':'')+parts.slice(0,index+1).join('/')}));
  if(!crumbs.length) crumbs.push({label:remotePath||'.',path:remotePath||'.'});
  container.innerHTML=crumbs.map((crumb,index)=>`<button data-path="${escapeHtml(crumb.path)}" class="${index===crumbs.length-1?'current':''}" title="${escapeHtml(crumb.path)}">${escapeHtml(crumb.label)}</button>`).join('<span>/</span>');
  container.querySelectorAll('button').forEach(button=>button.onclick=()=>loadFiles(button.dataset.path));
}

function fileType(name){
  const extension=name.includes('.')?name.split('.').pop().toUpperCase():'';
  return extension&&extension.length<=4?extension:'FILE';
}

function isImageFile(name){
  return /\.(png|jpe?g|gif|webp|bmp|svg|ico)$/i.test(name||'');
}

function isMarkdownFile(name){
  return /\.(md|markdown|mdown)$/i.test(name||'');
}

function isPdfFile(name){
  return /\.pdf$/i.test(name||'');
}

function openFileEntry(file){
  if(file.Directory) return loadFiles(file.Path);
  if(isImageFile(file.Name)) return openImageViewer(file);
  if(isMarkdownFile(file.Name)) return openMarkdownViewer(file);
  if(isPdfFile(file.Name)) return openPdfViewer(file);
  return openCodeEditor(file);
}

function formatFileTime(value){
  if(!value) return '--';
  const date=new Date(value);
  if(Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false});
}

async function openCodeEditor(file,systemdConfig=false){
  const dialog=$('#editorDialog');
  $('#editorFileName').textContent=file.Name;
  $('#editorPath').textContent=file.Path;
  $('#editorState').textContent='正在读取...';
  dialog.showModal();
  try{
    const content=await ReadRemoteFile(sessionId,file.Path);
    editorRemotePath=file.Path;
    editorSystemdConfig=systemdConfig;
    editorDirty=false;
    editorView?.destroy();
    editorView=new EditorView({
      doc:content,
      extensions:[
        basicSetup,
        EditorState.phrases.of({
          'Find':'查找','Replace':'替换','next':'下一个','previous':'上一个','all':'全选匹配',
          'match case':'区分大小写','regexp':'正则表达式','by word':'全词匹配','replace':'替换',
          'replace all':'全部替换','close':'关闭','current match':'当前匹配','on line':'位于第',
          'replaced match on line $':'已替换第 $ 行的匹配','replaced $ matches':'已替换 $ 处匹配'
        }),
        editorLanguage(file.Name),
        syntaxHighlighting(editorHighlightStyle),
        keymap.of([{key:'Mod-s',preventDefault:true,run:()=>{saveCodeEditor();return true}}]),
        EditorView.updateListener.of(update=>{
          if(update.docChanged){editorDirty=true;$('#editorState').textContent='未保存'}
        }),
        EditorView.theme({
          '&':{height:'100%',backgroundColor:'#0b0f16',color:'#d8dee9'},
          '.cm-content':{fontFamily:'Consolas, Cascadia Mono, monospace',fontSize:'13px',caretColor:'#73daca'},
          '.cm-gutters':{backgroundColor:'#101720',color:'#596579',border:'none'},
          '.cm-activeLine,.cm-activeLineGutter':{backgroundColor:'#151f2b'}
        },{dark:true})
      ],
      parent:$('#codeEditor')
    });
    $('#editorLanguage').textContent=editorLanguageName(file.Name);
    $('#editorState').textContent='已保存';
    editorView.focus();
  }catch(error){
    dialog.close();
    alert(String(error));
  }
}

let markdownFile=null;

function resolveMarkdownLink(href){
  if(/^(https?:|mailto:)/i.test(href)) return {external:href};
  if(href.startsWith('#')) return {hash:href.slice(1)};
  const clean=href.split('#')[0].split('?')[0];
  if(!clean||!isMarkdownFile(clean)) return null;
  const base=parentRemotePath(markdownFile?.Path||'/');
  const joined=clean.startsWith('/')?clean:pathJoinRemote(base,clean);
  return {path:joined.replace(/\/{2,}/g,'/')};
}

function pathJoinRemote(directory,name){
  const parts=[];
  `${directory}/${name}`.split('/').forEach(part=>{
    if(!part||part==='.') return;
    if(part==='..') parts.pop();
    else parts.push(part);
  });
  return `/${parts.join('/')}`;
}

async function openMarkdownViewer(file){
  markdownFile=file;
  const dialog=$('#markdownDialog');
  $('#markdownName').textContent=file.Name;
  $('#markdownPath').textContent=file.Path;
  $('#markdownBody').innerHTML='<p class="markdown-status">正在读取...</p>';
  if(!dialog.open) dialog.showModal();
  try{
    const content=await ReadRemoteFile(sessionId,file.Path);
    if(markdownFile?.Path!==file.Path) return;
    currentMarkdownHeading='';
    const body=$('#markdownBody');
    body.innerHTML=renderMarkdown(content)||'<p class="markdown-status">这个文件是空的</p>';
    renderMarkdownToc(collectMarkdownOutline(body));
    await renderMermaidBlocks(body);
    if(markdownFile?.Path!==file.Path) return;
    markCurrentMarkdownHeading();
  }catch(error){
    if(dialog.open&&markdownFile?.Path===file.Path) dialog.close();
    alert(String(error));
  }
}

function renderMarkdownToc(outline){
  const toc=$('#markdownToc');
  const layout=$('#markdownLayout');
  if(!outline.length){
    toc.hidden=true;
    layout.classList.add('no-toc');
    toc.innerHTML='';
    return;
  }
  const minLevel=Math.min(...outline.map(item=>item.level));
  toc.hidden=false;
  layout.classList.remove('no-toc');
  toc.innerHTML=`<b>目录</b>${outline.map(item=>`<button type="button" data-heading="${escapeHtml(item.id)}" style="padding-left:${8+(item.level-minLevel)*14}px">${escapeHtml(item.text)}</button>`).join('')}`;
}

let currentMarkdownHeading='';

function markCurrentMarkdownHeading(){
  const body=$('#markdownBody');
  const headings=[...body.querySelectorAll('h1,h2,h3,h4,h5,h6')];
  const buttons=[...$('#markdownToc').querySelectorAll('[data-heading]')];
  if(!headings.length||!buttons.length) return;
  const edge=body.getBoundingClientRect().top+16;
  let current=headings[0];
  for(const heading of headings){
    if(heading.getBoundingClientRect().top<=edge) current=heading;
  }
  if(current.id===currentMarkdownHeading) return;
  currentMarkdownHeading=current.id;
  buttons.forEach(button=>button.classList.toggle('active',button.dataset.heading===current.id));
  $('#markdownToc').querySelector('.active')?.scrollIntoView({block:'nearest'});
}

$('#markdownToc').addEventListener('click',event=>{
  const button=event.target.closest('[data-heading]');
  if(!button) return;
  $('#markdownBody').querySelector(`[id="${CSS.escape(button.dataset.heading)}"]`)?.scrollIntoView({behavior:'smooth',block:'start'});
});
$('#markdownBody').addEventListener('scroll',markCurrentMarkdownHeading);
$('#markdownClose').onclick=()=>$('#markdownDialog').close();
$('#markdownRefresh').onclick=()=>{if(markdownFile) openMarkdownViewer(markdownFile)};
$('#markdownEdit').onclick=()=>{
  const file=markdownFile;
  $('#markdownDialog').close();
  if(file) openCodeEditor(file);
};
$('#markdownDialog').addEventListener('cancel',event=>{
  event.preventDefault();
  if(!$('#flowOverlay').hidden){$('#flowOverlay').hidden=true;return}
  $('#markdownDialog').close();
});
$('#markdownClose').onclick=()=>{$('#flowOverlay').hidden=true;$('#markdownDialog').close()};
let flowScale=1;
let flowX=0;
let flowY=0;

function applyFlowTransform(){
  $('#flowStage').style.transform=`translate(${flowX}px, ${flowY}px) scale(${flowScale})`;
  const label=$('#flowZoomLabel');
  if(label) label.textContent=`${Math.round(flowScale*100)}%`;
}

function flowSvgSize(svg){
  const box=svg.viewBox&&svg.viewBox.baseVal;
  const width=box&&box.width?box.width:parseFloat(svg.getAttribute('width'))||svg.getBBox().width||800;
  const height=box&&box.height?box.height:parseFloat(svg.getAttribute('height'))||svg.getBBox().height||400;
  return {width,height};
}

function fitFlowchart(){
  const svg=$('#flowStage').querySelector('svg');
  const viewport=$('#flowViewport');
  if(!svg||!viewport) return;
  const size=flowSvgSize(svg);
  svg.style.width=`${size.width}px`;
  svg.style.height=`${size.height}px`;
  svg.style.maxWidth='none';
  flowScale=Math.max(0.2, Math.min((viewport.clientWidth-48)/size.width, (viewport.clientHeight-48)/size.height));
  flowX=(viewport.clientWidth-size.width*flowScale)/2;
  flowY=(viewport.clientHeight-size.height*flowScale)/2;
  applyFlowTransform();
}

function zoomFlowchart(next, clientX, clientY){
  const viewport=$('#flowViewport');
  const rect=viewport.getBoundingClientRect();
  const originX=clientX==null?rect.width/2:clientX-rect.left;
  const originY=clientY==null?rect.height/2:clientY-rect.top;
  const scale=Math.max(0.2, Math.min(next, 6));
  const ratio=scale/flowScale;
  flowX=originX-(originX-flowX)*ratio;
  flowY=originY-(originY-flowY)*ratio;
  flowScale=scale;
  applyFlowTransform();
}

function openFlowViewer(figure){
  const svg=figure.querySelector('svg');
  if(!svg) return;
  $('#flowStage').innerHTML='';
  const copy=svg.cloneNode(true);
  copy.style.maxWidth='none';
  $('#flowStage').append(copy);
  $('#flowOverlay').hidden=false;
  requestAnimationFrame(fitFlowchart);
}

$('#flowClose').onclick=()=>{$('#flowOverlay').hidden=true};
$('#flowFit').onclick=fitFlowchart;
$('#flowZoomIn').onclick=()=>zoomFlowchart(flowScale*1.25);
$('#flowZoomOut').onclick=()=>zoomFlowchart(flowScale/1.25);
$('#flowViewport').addEventListener('wheel',event=>{
  event.preventDefault();
  zoomFlowchart(flowScale*(event.deltaY<0?1.12:0.89), event.clientX, event.clientY);
},{passive:false});
$('#flowViewport').addEventListener('pointerdown',event=>{
  if(event.button!==0) return;
  const drag={x:event.clientX-flowX, y:event.clientY-flowY};
  event.currentTarget.classList.add('dragging');
  event.currentTarget.setPointerCapture(event.pointerId);
  const move=moveEvent=>{flowX=moveEvent.clientX-drag.x;flowY=moveEvent.clientY-drag.y;applyFlowTransform()};
  const up=()=>{event.currentTarget.classList.remove('dragging');event.currentTarget.removeEventListener('pointermove',move);event.currentTarget.removeEventListener('pointerup',up)};
  event.currentTarget.addEventListener('pointermove',move);
  event.currentTarget.addEventListener('pointerup',up);
});
$('#markdownBody').addEventListener('click',event=>{
  const flow=event.target.closest('.markdown-flow');
  if(flow){openFlowViewer(flow);return}
  const link=event.target.closest('#markdownBody a[href]');
  if(!link) return;
  event.preventDefault();
  const target=resolveMarkdownLink(link.getAttribute('href'));
  if(!target) return;
  if(target.external){window.open(target.external);return}
  if(target.hash){$('#markdownBody').querySelector(`[id="${CSS.escape(target.hash)}"]`)?.scrollIntoView();return}
  const name=target.path.split('/').pop();
  openMarkdownViewer({Name:name,Path:target.path});
});

async function openImageViewer(file){
  imagePreviewFile=file;
  imageZoom=1;
  $('#imageName').textContent=file.Name;
  $('#imageInfo').textContent='正在读取...';
  $('#imagePreview').removeAttribute('src');
  $('#imageDialog').showModal();
  try{
    const image=await ReadRemoteImage(sessionId,file.Path);
    const extension=file.Name.split('.').pop().toLowerCase();
    const mime={jpg:'image/jpeg',jpeg:'image/jpeg',png:'image/png',gif:'image/gif',webp:'image/webp',bmp:'image/bmp',svg:'image/svg+xml',ico:'image/x-icon'}[extension]||'application/octet-stream';
    $('#imagePreview').src=`data:${mime};base64,${image.Data}`;
    $('#imageInfo').textContent=formatBytes(image.Size);
    fitImagePreview();
  }catch(error){
    $('#imageDialog').close();
    alert(String(error));
  }
}

function fitImagePreview(){
  imageZoom=1;
  const image=$('#imagePreview');
  image.classList.add('fit');
  image.style.transform='none';
}

function actualImagePreview(){
  imageZoom=1;
  const image=$('#imagePreview');
  image.classList.remove('fit');
  image.style.transform='none';
}

function zoomImagePreview(change){
  imageZoom=Math.max(.25,Math.min(4,imageZoom+change));
  const image=$('#imagePreview');
  image.classList.remove('fit');
  image.style.transform=`scale(${imageZoom})`;
}

$('#imageFit').onclick=fitImagePreview;
$('#imageActual').onclick=actualImagePreview;
$('#imageZoomIn').onclick=()=>zoomImagePreview(.25);
$('#imageZoomOut').onclick=()=>zoomImagePreview(-.25);
$('#imageClose').onclick=()=>$('#imageDialog').close();
$('#imageDialog').addEventListener('cancel',event=>{event.preventDefault();$('#imageDialog').close()});
let pdfPreviewFile=null;
let pdfObjectURL='';

function closePdfViewer(){
  $('#pdfDialog').close();
  $('#pdfFrame').removeAttribute('src');
  if(pdfObjectURL){URL.revokeObjectURL(pdfObjectURL);pdfObjectURL=''}
}

async function openPdfViewer(file){
  pdfPreviewFile=file;
  $('#pdfName').textContent=file.Name;
  $('#pdfInfo').textContent='正在读取...';
  closePdfViewer();
  $('#pdfDialog').showModal();
  try{
    const pdf=await ReadRemotePDF(sessionId,file.Path);
    const bytes=Uint8Array.from(atob(pdf.Data),char=>char.charCodeAt(0));
    pdfObjectURL=URL.createObjectURL(new Blob([bytes],{type:'application/pdf'}));
    $('#pdfFrame').src=pdfObjectURL;
    $('#pdfInfo').textContent=formatBytes(pdf.Size);
  }catch(error){
    closePdfViewer();
    alert(String(error));
  }
}

$('#pdfClose').onclick=closePdfViewer;
$('#pdfDialog').addEventListener('cancel',event=>{event.preventDefault();closePdfViewer()});
$('#pdfDownload').onclick=async()=>{
  if(!pdfPreviewFile) return;
  const operationID=newTransferID();
  try{await DownloadFile(sessionId,pdfPreviewFile.Path,operationID)}catch(error){markTransferFailed(operationID,'download',error)}
};

$('#imageDownload').onclick=async()=>{
  if(!imagePreviewFile) return;
  const operationID=newTransferID();
  try{await DownloadFile(sessionId,imagePreviewFile.Path,operationID)}catch(error){markTransferFailed(operationID,'download',error)}
};

function editorLanguage(name){
  const extension=name.split('.').pop().toLowerCase();
  if(['js','jsx','ts','tsx','mjs','cjs'].includes(extension)) return javascript({typescript:['ts','tsx'].includes(extension),jsx:['jsx','tsx'].includes(extension)});
  if(extension==='json') return json();
  if(['html','htm','vue'].includes(extension)) return html();
  if(['css','scss','less'].includes(extension)) return css();
  if(['py','pyw'].includes(extension)) return python();
  if(extension==='go') return go();
  if(['java','kt','kts'].includes(extension)) return java();
  if(extension==='php') return php();
  if(extension==='rs') return rust();
  if(['sql','ddl'].includes(extension)) return sql();
  if(['xml','svg'].includes(extension)) return xml();
  if(['md','markdown'].includes(extension)) return markdown();
  if(['sh','bash','zsh','fish'].includes(extension)||['.bashrc','.profile','.zshrc'].includes(name.toLowerCase())) return StreamLanguage.define(shell);
  if(['yaml','yml'].includes(extension)) return StreamLanguage.define(yaml);
  return [];
}

function editorLanguageName(name){
  const extension=name.split('.').pop().toUpperCase();
  return extension&&extension!==name.toUpperCase()?extension:'纯文本';
}

async function saveCodeEditor(){
  if(!editorView||!editorRemotePath) return;
  $('#editorState').textContent='正在保存...';
  $('#editorSave').disabled=true;
  try{
    await SaveRemoteFile(sessionId,editorRemotePath,editorView.state.doc.toString());
    if(editorSystemdConfig) await SystemdDaemonReload(sessionId);
    editorDirty=false;
    $('#editorState').textContent='已保存';
    await loadFiles(currentPath);
    if(!$('#historyPanel').hidden) await loadFileHistory();
  }catch(error){
    $('#editorState').textContent='保存失败';
    alert(String(error));
  }finally{
    $('#editorSave').disabled=false;
  }
}

async function loadFileHistory(){
  const list=$('#historyList');
  list.innerHTML='<div class="history-empty">正在读取...</div>';
  try{
    const entries=await FileHistory(sessionId,editorRemotePath);
    if(!entries.length){
      list.innerHTML='<div class="history-empty">暂无历史记录<br><span>保存文件后自动生成</span></div>';
      return;
    }
    list.innerHTML=entries.map(entry=>`<div class="history-item" data-id="${escapeHtml(entry.ID)}"><div><b>${escapeHtml(entry.Kind)}</b><span>${new Date(entry.Time).toLocaleString('zh-CN')} · ${formatSize(entry.Size)}</span></div><div><button data-action="restore">恢复</button><button data-action="compare">对比</button></div></div>`).join('');
    list.querySelectorAll('.history-item').forEach(item=>{
      item.querySelector('[data-action="restore"]').onclick=event=>restoreFileHistory(item.dataset.id,event.currentTarget);
      item.querySelector('[data-action="compare"]').onclick=event=>compareFileHistory(item.dataset.id,event.currentTarget);
    });
  }catch(error){
    list.innerHTML=`<div class="history-empty">读取失败<br><span>${escapeHtml(String(error))}</span></div>`;
  }
}

async function compareFileHistory(historyID,button){
  button.disabled=true;
  try{
    const historical=await ReadFileHistory(sessionId,editorRemotePath,historyID);
    const current=editorView.state.doc.toString();
    const parts=diffLines(historical,current);
    $('#diffContent').innerHTML=parts.map(part=>`<span class="${part.added?'diff-added':part.removed?'diff-removed':'diff-same'}">${escapeHtml(part.value)}</span>`).join('');
    $('#diffPanel').hidden=false;
  }catch(error){
    alert(String(error));
  }finally{
    button.disabled=false;
  }
}

async function refreshCodeEditor(){
  if(!editorView||!editorRemotePath) return;
  $('#editorRefresh').disabled=true;
  $('#editorState').textContent='正在刷新...';
  try{
    if(editorDirty) await RecordFileHistory(sessionId,editorRemotePath,editorView.state.doc.toString(),'刷新前草稿');
    const content=await ReadRemoteFile(sessionId,editorRemotePath);
    editorView.dispatch({changes:{from:0,to:editorView.state.doc.length,insert:content}});
    editorDirty=false;
    $('#editorState').textContent='已从服务器刷新';
    if(!$('#historyPanel').hidden) await loadFileHistory();
  }catch(error){
    $('#editorState').textContent='刷新失败';
    alert(String(error));
  }finally{
    $('#editorRefresh').disabled=false;
  }
}

async function restoreFileHistory(historyID,button){
  if(editorDirty&&!confirm('当前有未保存的修改，确定用历史版本替换吗？')) return;
  button.disabled=true;
  try{
    const content=await ReadFileHistory(sessionId,editorRemotePath,historyID);
    editorView.dispatch({changes:{from:0,to:editorView.state.doc.length,insert:content}});
    editorDirty=true;
    $('#editorState').textContent='已恢复，尚未保存';
    editorView.focus();
  }catch(error){
    alert(String(error));
  }finally{
    button.disabled=false;
  }
}

async function closeCodeEditor(){
  if(editorDirty&&editorView&&editorRemotePath){
    try{
      await RecordFileHistory(sessionId,editorRemotePath,editorView.state.doc.toString(),'未保存草稿');
    }catch(error){
      alert(`保存关闭历史失败：${error}`);
      return;
    }
  }
  $('#editorDialog').close();
  editorView?.destroy();
  editorView=null;
  editorRemotePath='';
  editorSystemdConfig=false;
  editorDirty=false;
}

$('#editorSave').onclick=saveCodeEditor;
$('#editorRefresh').onclick=refreshCodeEditor;
$('#editorHistory').onclick=async()=>{
  const panel=$('#historyPanel');
  panel.hidden=!panel.hidden;
  if(!panel.hidden) await loadFileHistory();
};
$('#historyClose').onclick=()=>{$('#historyPanel').hidden=true};
$('#diffClose').onclick=()=>{$('#diffPanel').hidden=true};
$('#editorClose').onclick=closeCodeEditor;
$('#editorDialog').addEventListener('cancel',event=>{event.preventDefault();closeCodeEditor()});

$('#refresh').onclick=()=>loadFiles(currentPath);
$('#pathGo').onclick=()=>jumpToPath();
$('#pathJumpClose').onclick=()=>document.querySelector('.path-jump').close();
$('#pathToggle').onclick=event=>{
  event.stopPropagation();
  const panel=document.querySelector('.path-jump');
  if(panel.open){panel.close();return}
  panel.showModal();
  const toolbar=$('.bookmarkbar').getBoundingClientRect();
  const button=$('#pathToggle').getBoundingClientRect();
  panel.style.top=`${toolbar.bottom+6}px`;
  panel.style.left=`${Math.max(8,Math.min(button.left,window.innerWidth-panel.offsetWidth-8))}px`;
  $('#pathInput').focus();
  $('#pathInput').select();
};
$('#pathInput').addEventListener('keydown',event=>{
  if(event.key==='Enter'){event.preventDefault();jumpToPath()}
  if(event.key==='Escape') document.querySelector('.path-jump').close();
});
function jumpToPath(){
  const target=$('#pathInput').value.trim();
  if(target){document.querySelector('.path-jump').close();loadFiles(target)}
}
document.querySelector('.path-jump').addEventListener('cancel',event=>{event.preventDefault();document.querySelector('.path-jump').close()});
document.querySelector('.path-jump').addEventListener('pointerdown',event=>{if(event.target===document.querySelector('.path-jump'))document.querySelector('.path-jump').close()});

function saveDirectoryBookmarks(){
  localStorage.setItem('nekssh.desktop.directoryBookmarks',JSON.stringify(directoryBookmarks));
  renderDirectoryBookmarks();
}

function renderDirectoryBookmarks(){
  const menu=$('#bookmarkMenu');
  if(!directoryBookmarks.length){
    menu.innerHTML='<div class="bookmark-empty">暂无快捷目录</div>';
    return;
  }
  menu.innerHTML=directoryBookmarks.map((bookmark,index)=>`<div class="bookmark-item"><button data-open="${index}" title="${escapeHtml(bookmark)}"><b>${escapeHtml(bookmark.split('/').filter(Boolean).pop()||'根目录')}</b><span>${escapeHtml(bookmark)}</span></button><button data-remove="${index}" title="删除">×</button></div>`).join('');
  menu.querySelectorAll('[data-open]').forEach(button=>button.onclick=()=>{closeBookmarkMenu();loadFiles(directoryBookmarks[Number(button.dataset.open)])});
  menu.querySelectorAll('[data-remove]').forEach(button=>button.onclick=event=>{event.stopPropagation();directoryBookmarks.splice(Number(button.dataset.remove),1);saveDirectoryBookmarks()});
}

$('#saveBookmark').onclick=()=>{
  if(!currentPath||directoryBookmarks.includes(currentPath)) return;
  directoryBookmarks.unshift(currentPath);
  directoryBookmarks=directoryBookmarks.slice(0,30);
  saveDirectoryBookmarks();
};
$('#bookmarkButton').onclick=event=>{
  event.stopPropagation();
  const menu=$('#bookmarkMenu');
  if(menu.open){closeBookmarkMenu();return}
  renderDirectoryBookmarks();
  menu.showModal();
};
function closeBookmarkMenu(){const menu=$('#bookmarkMenu');if(menu.open)menu.close()}
$('#bookmarkMenu').addEventListener('cancel',event=>{event.preventDefault();closeBookmarkMenu()});
$('#bookmarkMenu').addEventListener('pointerdown',event=>{if(event.target===$('#bookmarkMenu'))closeBookmarkMenu()});
$('#upDirectory').onclick=()=>{
  if(currentPath==='/') return;
  const parent=currentPath.replace(/\/+$/,'').replace(/\/[^/]*$/,'')||'/';
  loadFiles(parent);
};
$('#upload').onclick=async()=>{
  const operationID=newTransferID();
  try{await UploadFile(sessionId,currentPath,operationID);await loadFiles(currentPath)}catch(error){markTransferFailed(operationID,'upload',error)}
};
async function uploadDroppedFiles(paths){
  if(!sessionId||!paths?.length) return;
  for(const localPath of paths){
    const operationID=newTransferID();
    try{await UploadLocalFile(sessionId,currentPath,localPath,operationID)}catch(error){markTransferFailed(operationID,'upload',error)}
  }
  try{await loadFiles(currentPath)}catch(error){console.warn(error)}
}
OnFileDrop((_x,_y,paths)=>uploadDroppedFiles(paths),true);
$('#mkdir').onclick=async()=>{
  const name=prompt('新目录名称');
  if(!name) return;
  try{await MakeDirectory(sessionId,currentPath,name);await loadFiles(currentPath)}catch(error){alert(String(error))}
};

async function fileMenu(event,file){
  event.preventDefault();
  event.stopPropagation();
  $('#fileList').querySelectorAll('.file.selected').forEach(item=>item.classList.remove('selected'));
  event.currentTarget.classList.add('selected');
  const menu=$('#fileContextMenu');
  let elf=false;
  if(!file.Directory){try{elf=await IsELF(sessionId,file.Path)}catch(error){console.warn(error)}}
  const primary=file.Directory
    ?'<button data-action="open"><i>↳</i><span>打开目录</span></button>'
    :isImageFile(file.Name)?'<button data-action="preview"><i>▣</i><span>查看图片</span></button>'
    :isMarkdownFile(file.Name)?'<button data-action="markdown"><i>M</i><span>Markdown 预览</span></button><button data-action="edit"><i>&lt;/&gt;</i><span>代码编辑</span></button>'
    :isPdfFile(file.Name)?'<button data-action="pdf"><i>P</i><span>查看 PDF</span></button>'
    :'<button data-action="edit"><i>&lt;/&gt;</i><span>代码编辑</span></button>';
  menu.innerHTML=`<div class="context-file"><b>${escapeHtml(file.Name)}</b><span>${file.Directory?'目录':formatSize(file.Size)}</span></div>${primary}<button data-action="download"><i>↓</i><span>下载</span></button>${elf?'<button data-action="createService"><i>●</i><span>创建 systemd 服务</span></button>':''}<button data-action="permission"><i>◆</i><span>权限与所有者</span></button><button data-action="copy"><i>□</i><span>复制完整路径</span></button><button data-action="refresh"><i>↻</i><span>刷新目录</span></button><div class="context-separator"></div><button data-action="delete" class="danger"><i>×</i><span>删除</span></button>`;
  menu.hidden=false;
  const width=menu.offsetWidth,height=menu.offsetHeight;
  menu.style.left=`${Math.max(8,Math.min(event.clientX,window.innerWidth-width-8))}px`;
  menu.style.top=`${Math.max(8,Math.min(event.clientY,window.innerHeight-height-8))}px`;
  menu.querySelectorAll('[data-action]').forEach(button=>button.onclick=async()=>{
    menu.hidden=true;
    const action=button.dataset.action;
    if(action==='open') return loadFiles(file.Path);
    if(action==='edit') return openCodeEditor(file);
    if(action==='preview') return openImageViewer(file);
    if(action==='markdown') return openMarkdownViewer(file);
    if(action==='pdf') return openPdfViewer(file);
    if(action==='createService') return openCreateService(file);
    if(action==='permission') return openPermissionDialog(file);
    if(action==='refresh') return loadFiles(currentPath);
    if(action==='copy'){
      try{await navigator.clipboard.writeText(file.Path)}catch(error){alert(`复制失败：${error}`)}
      return;
    }
    if(action==='download'){
      const operationID=newTransferID();
      try{file.Directory?await DownloadDirectory(sessionId,file.Path,operationID):await DownloadFile(sessionId,file.Path,operationID)}catch(error){markTransferFailed(operationID,'download',error)}
      return;
    }
    if(action==='delete'&&confirm(`删除 ${file.Name}？${file.Directory?'\n目录内容也会一并删除。':''}`)){
      try{await DeletePath(sessionId,file.Path);await loadFiles(currentPath)}catch(error){alert(String(error))}
    }
  });
}

function openCreateService(file){
  serviceExecutableFile=file;
  $('#serviceExecutable').textContent=file.Path;
  $('#serviceName').value=file.Name.replace(/[^A-Za-z0-9_.@-]/g,'-').replace(/^-+|-+$/g,'')||'my-service';
  $('#serviceDescription').value=file.Name;
  $('#serviceArguments').value='';
  $('#serviceUser').value='root';
  $('#serviceWorkingDirectory').value=parentRemotePath(file.Path);
  $('#serviceEnableNow').checked=true;
  $('#createServiceDialog').showModal();
  $('#serviceName').focus();
}

async function createSystemService(){
  if(!serviceExecutableFile) return;
  const request={Name:$('#serviceName').value.trim(),Description:$('#serviceDescription').value.trim(),Executable:serviceExecutableFile.Path,Arguments:$('#serviceArguments').value.trim(),User:$('#serviceUser').value.trim(),WorkingDirectory:$('#serviceWorkingDirectory').value.trim(),EnableNow:$('#serviceEnableNow').checked};
  if(!request.Name){alert('请输入服务名称');return}
  $('#createServiceSave').disabled=true;
  try{await CreateSystemdService(sessionId,request);$('#createServiceDialog').close();alert('systemd 服务创建成功')}catch(error){alert(String(error))}finally{$('#createServiceSave').disabled=false}
}

async function openServiceManager(){
  $('#serviceManagerDialog').showModal();
  $('#serviceSearch').value='';
  await loadSystemdServices();
  $('#serviceSearch').focus();
}

async function loadSystemdServices(){
  const list=$('#serviceList');
  list.innerHTML='<div class="service-empty">正在读取服务...</div>';
  try{systemdServiceItems=await SystemdServices(sessionId);renderSystemdServices()}catch(error){list.innerHTML=`<div class="service-empty">${escapeHtml(String(error))}</div>`}
}

function renderSystemdServices(){
  const query=$('#serviceSearch').value.trim().toLowerCase();
  const services=systemdServiceItems.filter(service=>!query||`${service.Name} ${service.Description}`.toLowerCase().includes(query));
  const list=$('#serviceList');
  if(!services.length){list.innerHTML='<div class="service-empty">暂无匹配服务</div>';return}
  list.innerHTML=services.map(service=>`<div class="service-item"><i class="${service.Active==='active'?'active':''}"></i><div><b>${escapeHtml(service.Name)}</b><span>${escapeHtml(service.Description)} · ${escapeHtml(service.Active)}/${escapeHtml(service.Sub)}</span></div><button data-service-config="${escapeHtml(service.Name)}">配置</button><button data-service-action="start" data-service="${escapeHtml(service.Name)}">启动</button><button data-service-action="stop" data-service="${escapeHtml(service.Name)}">停止</button><button data-service-action="restart" data-service="${escapeHtml(service.Name)}">重启</button></div>`).join('');
  list.querySelectorAll('[data-service-config]').forEach(button=>button.onclick=async()=>{
    button.disabled=true;
    try{
      const name=button.dataset.serviceConfig;
      const filePath=await SystemdServicePath(sessionId,name);
      $('#serviceManagerDialog').close();
      await openCodeEditor({Name:name,Path:filePath},true);
    }catch(error){alert(String(error))}finally{button.disabled=false}
  });
  list.querySelectorAll('[data-service-action]').forEach(button=>button.onclick=async()=>{
    button.disabled=true;
    try{await SystemdServiceAction(sessionId,button.dataset.service,button.dataset.serviceAction);await loadSystemdServices()}catch(error){alert(String(error))}finally{button.disabled=false}
  });
}

$('#createServiceSave').onclick=createSystemService;
$('#createServiceClose').onclick=$('#createServiceCancel').onclick=()=>$('#createServiceDialog').close();
$('#createServiceDialog').addEventListener('cancel',event=>{event.preventDefault();$('#createServiceDialog').close()});
$('#serviceManagerClose').onclick=()=>$('#serviceManagerDialog').close();
$('#serviceManagerDialog').addEventListener('cancel',event=>{event.preventDefault();$('#serviceManagerDialog').close()});
$('#serviceRefresh').onclick=loadSystemdServices;
$('#serviceSearch').addEventListener('input',renderSystemdServices);

async function openPermissionDialog(file){
  permissionTarget=file;
  $('#permissionPath').textContent=file.Path;
  ensureSelectValue($('#permissionMode'),modeToOctal(file.Mode),`${modeToOctal(file.Mode)} · 当前权限`);
  $('#permissionOwner').innerHTML=`<option value="${escapeHtml(file.Owner)}">${escapeHtml(file.Owner)} · 当前所有者</option>`;
  $('#permissionGroup').innerHTML=`<option value="${escapeHtml(file.Group)}">${escapeHtml(file.Group)} · 当前用户组</option>`;
  $('#permissionDialog').showModal();
  $('#permissionMode').focus();
  try{
    const options=await OwnershipOptions(sessionId);
    fillIdentitySelect($('#permissionOwner'),options.Users,file.Owner,'当前所有者');
    fillIdentitySelect($('#permissionGroup'),options.Groups,file.Group,'当前用户组');
  }catch(error){
    console.warn(error);
  }
}

function ensureSelectValue(select,value,label){
  if(![...select.options].some(option=>option.value===String(value))){
    select.add(new Option(label,String(value)));
  }
  select.value=String(value);
}

function fillIdentitySelect(select,options,current,label){
  const values=new Map((options||[]).map(option=>[String(option.ID),`${option.Name} · ${option.ID}`]));
  if(!values.has(String(current))) values.set(String(current),`${current} · ${label}`);
  select.innerHTML=[...values.entries()].sort((a,b)=>Number(a[0])-Number(b[0])).map(([id,text])=>`<option value="${escapeHtml(id)}">${escapeHtml(text)}</option>`).join('');
  select.value=String(current);
}

function modeToOctal(mode){
  const permissions=(mode||'').slice(-9);
  if(permissions.length!==9) return '';
  let result='';
  for(let index=0;index<9;index+=3){
    let value=0;
    if(permissions[index]==='r') value+=4;
    if(permissions[index+1]==='w') value+=2;
    if(['x','s','t'].includes(permissions[index+2])) value+=1;
    result+=value;
  }
  return result;
}

async function applyPermission(){
  if(!permissionTarget) return;
  const mode=$('#permissionMode').value.trim();
  const owner=$('#permissionOwner').value.trim();
  const group=$('#permissionGroup').value.trim();
  $('#permissionSave').disabled=true;
  try{
    await SetPathMode(sessionId,permissionTarget.Path,mode);
    await SetPathOwner(sessionId,permissionTarget.Path,owner,group);
    $('#permissionDialog').close();
    await loadFiles(currentPath);
  }catch(error){alert(String(error))}finally{$('#permissionSave').disabled=false}
}

$('#permissionSave').onclick=applyPermission;
$('#permissionClose').onclick=$('#permissionCancel').onclick=()=>$('#permissionDialog').close();
$('#permissionDialog').addEventListener('cancel',event=>{event.preventDefault();$('#permissionDialog').close()});

function initWorkspaceShortcuts(){
  const list=$('#fileList');
  document.addEventListener('keydown',event=>{
    if(!(event.ctrlKey||event.metaKey)||event.altKey) return;
    if(event.key==='1'){
      event.preventDefault();event.stopPropagation();
      if(sessionId){term.focus();$('#terminal').classList.add('keyboard-active');list.classList.remove('keyboard-active')}
    }
    if(event.key==='2'){
      event.preventDefault();event.stopPropagation();
      if(sessionId){list.focus();list.classList.add('keyboard-active');$('#terminal').classList.remove('keyboard-active');selectFileByOffset(0)}
    }
  },true);
  list.addEventListener('keydown',event=>{
    if(event.key==='ArrowDown'||event.key==='ArrowUp'){
      event.preventDefault();selectFileByOffset(event.key==='ArrowDown'?1:-1);
    }else if(event.key==='Enter'){
      event.preventDefault();
      const file=list.querySelector('.file.selected')?._file;
      if(file) openFileEntry(file);
    }else if(event.key==='Backspace'){
      event.preventDefault();
      if(currentPath!=='/') loadFiles(parentRemotePath(currentPath));
    }
  });
  list.addEventListener('blur',()=>list.classList.remove('keyboard-active'));
  $('#terminal').addEventListener('pointerdown',()=>{$('#terminal').classList.add('keyboard-active');list.classList.remove('keyboard-active')});

  function selectFileByOffset(offset){
    const rows=[...list.querySelectorAll('.file:not(.head)')].filter(row=>!row.hidden&&row._file);
    if(!rows.length) return;
    let index=rows.indexOf(list.querySelector('.file.selected'));
    if(index<0) index=0; else if(offset) index=Math.max(0,Math.min(rows.length-1,index+offset));
    list.querySelectorAll('.file.selected').forEach(row=>row.classList.remove('selected'));
    rows[index].classList.add('selected');
    scrollFileRowIntoView(rows[index]);
  }
}

document.addEventListener('pointerdown',event=>{if(!event.target.closest('#fileContextMenu'))$('#fileContextMenu').hidden=true});
document.addEventListener('keydown',event=>{if(event.key==='Escape')$('#fileContextMenu').hidden=true});
window.addEventListener('blur',()=>{$('#fileContextMenu').hidden=true});
document.addEventListener('pointerdown',event=>{if(!event.target.closest('#tabContextMenu'))$('#tabContextMenu').hidden=true});
document.addEventListener('keydown',event=>{if(event.key==='Escape')$('#tabContextMenu').hidden=true});
window.addEventListener('blur',()=>{$('#tabContextMenu').hidden=true});

function initFileResize(){
  const workspace=$('#workspace'), files=$('#files');
  const handle=document.createElement('div');
  handle.id='fileResizeHandle';
  handle.title='拖动调整文件列表高度';
  workspace.insertBefore(handle,files);
  const savedHeight=Number(localStorage.getItem('nekssh.desktop.files.height')||0);
  let savedApplied=false;
  new ResizeObserver(()=>{
    if(!savedApplied&&savedHeight>0&&workspace.getBoundingClientRect().height>0){
      savedApplied=true;
      setFilePanelHeight(savedHeight);
    }
  }).observe(workspace);
  handle.addEventListener('pointerdown',event=>{
    event.preventDefault();
    handle.setPointerCapture(event.pointerId);
    document.body.classList.add('resizing-files');
    const move=moveEvent=>{
      const rect=workspace.getBoundingClientRect();
      setFilePanelHeight(rect.bottom-moveEvent.clientY);
    };
    const up=upEvent=>{
      handle.releasePointerCapture(upEvent.pointerId);
      document.body.classList.remove('resizing-files');
      localStorage.setItem('nekssh.desktop.files.height',String(Math.round(files.getBoundingClientRect().height)));
      handle.removeEventListener('pointermove',move);
      handle.removeEventListener('pointerup',up);
      handle.removeEventListener('pointercancel',up);
      fit.fit();
    };
    handle.addEventListener('pointermove',move);
    handle.addEventListener('pointerup',up);
    handle.addEventListener('pointercancel',up);
  });
}

function setFilePanelHeight(height){
  const workspace=$('#workspace');
  const total=workspace.getBoundingClientRect().height;
  if(!total) return;
  const handleHeight=6, minTerminal=160, minFiles=90;
  const maxFiles=Math.max(minFiles,total-handleHeight-minTerminal);
  const filesHeight=Math.min(Math.max(height,minFiles),maxFiles);
  workspace.style.gridTemplateRows=`minmax(${minTerminal}px, 1fr) ${handleHeight}px ${filesHeight}px`;
  fit.fit();
}

function initSearchBox(){
  const panel=document.createElement('div');
  panel.id='globalSearch';
  panel.hidden=true;
  panel.innerHTML='<input id="globalSearchInput" autocomplete="off"><button id="searchPrev" title="上一个">↑</button><button id="searchNext" title="下一个">↓</button><button id="searchClose" title="关闭">×</button><span id="searchInfo"></span>';
  document.body.append(panel);
  const input=$('#globalSearchInput'), info=$('#searchInfo');
  let fileMatchIndex=-1;
  let scope='terminal';
  let lastSearchScope='terminal';
  const options={caseSensitive:false,decorations:{matchBackground:'#3d4f2f',activeMatchBackground:'#d7ba7d',matchOverviewRuler:'#35b8a0',activeMatchColorOverviewRuler:'#d7ba7d'}};
  const openSearch=nextScope=>{
    scope=nextScope;
    input.placeholder=scope==='files'?'搜索当前目录':'搜索终端';
    panel.hidden=false;
    input.focus();
    input.select();
    runSearch('same');
  };
  const runSearch=direction=>{
    const value=input.value.trim();
    if(scope==='files'){
      clearTerminalSearch();
      const fileCount=applyFileSearch(value,direction);
      info.textContent=value?`文件 ${fileCount}`:'';
      if(value&&fileCount===0) info.textContent='无匹配';
      return;
    }
    applyFileSearch('','same');
    if(!value){clearTerminalSearch();info.textContent='';return}
    const count=applyTerminalSearch(value,direction);
    info.textContent=count?'终端 '+(terminalMatchIndex+1)+'/'+count:'无匹配';
  };
  const keyHandler=event=>{
    if(event.target.closest?.('#editorDialog')) return;
    if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='f'){
      event.preventDefault();
      event.stopPropagation();
      openSearch(event.target.closest('#files')?'files':lastSearchScope);
    }else if(event.key==='Escape'&&!panel.hidden){
      closeSearch();
    }
  };
  term.attachCustomKeyEventHandler(event=>{
    if(event.type!=='keydown') return true;
    const control=event.ctrlKey||event.metaKey;
    const key=event.key.toLowerCase();
    if(terminalCopyMode&&key==='escape'){
      event.preventDefault();
      setTerminalCopyMode(false);
      return false;
    }
    if(control&&key==='f'){
      event.preventDefault();
      openSearch('terminal');
      return false;
    }
    if(control&&key==='r'){
      event.preventDefault();
      openCommandHistory();
      return false;
    }
    if(control&&key==='c'&&(term.hasSelection()||event.shiftKey)){
      event.preventDefault();
      const selected=term.getSelection();
      if(selected) ClipboardSetText(selected);
      return false;
    }
    if(control&&key==='v'){
      event.preventDefault();
      const targetSession=sessionId;
      ClipboardGetText().then(text=>{if(text&&targetSession&&targetSession===sessionId){trackTerminalInput(targetSession,text);Input(targetSession,text)}}).catch(error=>console.warn('读取剪贴板失败',error));
      return false;
    }
    return true;
  });
  $('#terminal').addEventListener('pointerdown',()=>{lastSearchScope='terminal'});
  $('#files').addEventListener('pointerdown',()=>{lastSearchScope='files'});
  document.addEventListener('keydown',keyHandler,true);
  input.addEventListener('input',()=>{fileMatchIndex=-1;terminalMatchIndex=-1;terminalMatches=[];runSearch('same')});
  input.addEventListener('keydown',event=>{
    if(event.key==='ArrowDown'||event.key==='ArrowUp'){
      event.preventDefault();
      runSearch(event.key==='ArrowUp'?'prev':'next');
    }else if(event.key==='Enter'){
      event.preventDefault();
      runSearch(event.shiftKey?'prev':'next');
    }
  });
  $('#searchPrev').onclick=()=>runSearch('prev');
  $('#searchNext').onclick=()=>runSearch('next');
  $('#searchClose').onclick=closeSearch;
  new MutationObserver(()=>{if(!panel.hidden&&scope==='files'){fileMatchIndex=-1;applyFileSearch(input.value.trim(),'same')}}).observe($('#fileList'),{childList:true});
  function closeSearch(){
    const selectedFile=scope==='files'?$('#fileList .file-match-active'):null;
    panel.hidden=true;
    input.value='';
    info.textContent='';
    applyFileSearch('','same');
    if(selectedFile){
      $('#fileList').querySelectorAll('.file.selected').forEach(row=>row.classList.remove('selected'));
      selectedFile.classList.add('selected');
      scrollFileRowIntoView(selectedFile);
    }
    clearTerminalSearch();
    if(scope==='terminal') term.focus();
    else{$('#fileList').focus();$('#fileList').classList.add('keyboard-active')}
  }
  function applyFileSearch(query,direction){
    const needle=query.toLowerCase();
    const matches=[];
    for(const row of document.querySelectorAll('#fileList .file:not(.head)')){
      const matched=!!needle&&(row.dataset.search||row.textContent).toLowerCase().includes(needle);
      row.hidden=!!needle&&!matched;
      row.classList.remove('file-match-active');
      if(matched) matches.push(row);
    }
    if(!needle){fileMatchIndex=-1;return countVisibleFileRows()}
    if(matches.length===0){fileMatchIndex=-1;return 0}
    if(direction==='next') fileMatchIndex=(fileMatchIndex+1)%matches.length;
    else if(direction==='prev') fileMatchIndex=(fileMatchIndex-1+matches.length)%matches.length;
    else if(fileMatchIndex<0||fileMatchIndex>=matches.length) fileMatchIndex=0;
    const active=matches[fileMatchIndex];
    $('#fileList').querySelectorAll('.file.selected').forEach(row=>row.classList.remove('selected'));
    active.classList.add('selected');
    active.classList.add('file-match-active');
    scrollFileRowIntoView(active);
    return matches.length;
  }
}

function applyTerminalSearch(query,direction){
  const needle=query.toLowerCase();
  clearTerminalSearch();
  if(!needle){
    terminalMatches=[];
    terminalMatchIndex=-1;
    return 0;
  }
  terminalMatches=collectTerminalMatches(needle);
  if(terminalMatches.length===0){
    terminalMatchIndex=-1;
    return 0;
  }
  if(direction==='next') terminalMatchIndex=(terminalMatchIndex+1)%terminalMatches.length;
  else if(direction==='prev') terminalMatchIndex=(terminalMatchIndex-1+terminalMatches.length)%terminalMatches.length;
  else if(terminalMatchIndex<0||terminalMatchIndex>=terminalMatches.length) terminalMatchIndex=0;
  const match=terminalMatches[terminalMatchIndex];
  selectTerminalMatch(match.row,match.col,query.length);
  return terminalMatches.length;
}

function collectTerminalMatches(needle){
  const buffer=term.buffer.active;
  const matches=[];
  for(let row=0;row<buffer.length;row++){
    const text=(buffer.getLine(row)?.translateToString(true)||'').toLowerCase();
    let col=text.indexOf(needle);
    while(col>=0){
      matches.push({row,col});
      col=text.indexOf(needle,col+Math.max(needle.length,1));
    }
  }
  const viewportTop=buffer.viewportY;
  const cursorRow=buffer.baseY+buffer.cursorY;
  matches.sort((a,b)=>Math.abs(a.row-cursorRow)-Math.abs(b.row-cursorRow)||a.row-b.row||a.col-b.col);
  const firstVisible=matches.findIndex(match=>match.row>=viewportTop&&match.row<viewportTop+term.rows);
  if(firstVisible>0) matches.unshift(...matches.splice(firstVisible,1));
  return matches;
}

function selectTerminalMatch(row,col,length){
  term.scrollToLine(row);
  requestAnimationFrame(()=>term.select(col,row,length));
}

function clearTerminalSearch(){
  search.clearDecorations();
  term.clearSelection();
}

function countVisibleFileRows(){
  return [...document.querySelectorAll('#fileList .file:not(.head)')].filter(row=>!row.hidden).length;
}

function startStatsRefresh(displayName){
  stopStatsRefresh();
  $('.server-name').textContent=displayName||'当前服务器';
  statsDelay=window.setTimeout(loadServerStats,4000);
  statsTimer=window.setInterval(loadServerStats,15000);
}

function stopStatsRefresh(){
  if(statsTimer){
    clearInterval(statsTimer);
    statsTimer=0;
  }
  if(statsDelay){
    clearTimeout(statsDelay);
    statsDelay=0;
  }
  statsFlight=null;
}

let statsDelay=0;

let statsFlight=null;

async function loadServerStats(){
  if(!sessionId||statsFlight===sessionId) return;
  const requestedSession=sessionId;
  statsFlight=requestedSession;
  try{
    const stats=await ServerStats(requestedSession);
    if(requestedSession!==sessionId) return;
    const processes=await ServerProcesses(requestedSession);
    if(requestedSession!==sessionId) return;
    const partitions=await ServerPartitions(requestedSession);
    if(requestedSession!==sessionId) return;
    if(stats.Host) $('.server-name').textContent=stats.Host;
    updateResource('cpu',stats.CPUPercent,`${stats.CPUPercent.toFixed(1)}%`);
    updateResource('mem',percent(stats.MemoryUsed,stats.MemoryTotal),`${formatBytes(stats.MemoryUsed)} / ${formatBytes(stats.MemoryTotal)}`);
    updateResource('disk',percent(stats.DiskUsed,stats.DiskTotal),`${formatBytes(stats.DiskUsed)} / ${formatBytes(stats.DiskTotal)}`);
    updateResource('swap',percent(stats.SwapUsed,stats.SwapTotal),`${formatBytes(stats.SwapUsed)} / ${formatBytes(stats.SwapTotal)}`);
    renderProcesses(processes);
    renderPartitions(partitions);
  }catch(error){
    $('.server-name').textContent='资源信息不可用';
    $('#processList').innerHTML='<div class="panel-empty">进程信息不可用</div>';
    $('#partitionList').innerHTML='<div class="panel-empty">分区信息不可用</div>';
  }finally{
    if(statsFlight===requestedSession) statsFlight=null;
  }
}

function newTransferID(){
  return `transfer-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function markTransferFailed(id,direction,error){
  const existing=transfers.get(id);
  if(existing){
    transfers.set(id,{...existing,Status:'失败',Error:String(error)});
    renderTransfers();
  }else if(String(error)){
    alert(String(error));
  }
}

function renderTransfers(){
  const list=$('#transferList');
  const items=[...transfers.values()].filter(item=>item.Direction===transferTab).reverse();
  const active=[...transfers.values()].filter(item=>item.Status==='进行中').length;
  $('#transferBadge').hidden=active===0;
  $('#transferBadge').textContent=active;
  if(!items.length){
    list.innerHTML=`<div class="transfer-empty">暂无${transferTab==='upload'?'上传':'下载'}任务</div>`;
    return;
  }
  list.innerHTML=items.map(item=>{
    const percent=item.Total?Math.min(100,item.Transferred*100/item.Total):0;
    const status=item.Status==='失败'?'失败':item.Status==='已完成'?'已完成':`${percent.toFixed(1)}%`;
    const locationButton=item.Direction==='download'&&item.Status==='已完成'&&item.LocalPath?`<button class="show-in-folder" data-path="${escapeHtml(item.LocalPath)}">打开目录</button>`:'';
    return `<div class="transfer-item" data-status="${item.Status}"><div class="transfer-info"><b title="${escapeHtml(item.Name)}">${escapeHtml(item.Name)}</b><span>${status}</span></div><div class="transfer-progress"><i style="width:${percent}%"></i></div><div class="transfer-detail"><span>${formatBytes(item.Transferred)} / ${formatBytes(item.Total)}</span><span>${item.Direction==='upload'?'上传':'下载'} ${locationButton}</span></div></div>`;
  }).join('');
  list.querySelectorAll('.show-in-folder').forEach(button=>button.onclick=async()=>{
    try{await ShowInFolder(button.dataset.path)}catch(error){alert(String(error))}
  });
}

$('#transferButton').onclick=event=>{event.stopPropagation();$('#transferPanel').hidden=!$('#transferPanel').hidden;renderTransfers()};
document.addEventListener('pointerdown',event=>{if(!event.target.closest('#transferPanel')&&!event.target.closest('#transferButton'))$('#transferPanel').hidden=true});
document.querySelectorAll('[data-transfer-tab]').forEach(button=>button.onclick=()=>{
  transferTab=button.dataset.transferTab;
  document.querySelectorAll('[data-transfer-tab]').forEach(tab=>tab.classList.toggle('active',tab===button));
  renderTransfers();
});
$('#clearTransfers').onclick=()=>{
  for(const [id,item] of transfers) if(item.Status!=='进行中') transfers.delete(id);
  renderTransfers();
};

function resetStatsPanel(){
  $('.server-name').textContent='未连接服务器';
  for(const id of ['cpu','mem','disk','swap']) updateResource(id,0,'--');
  $('#processList').innerHTML='<div class="panel-empty">连接后显示进程</div>';
  processItems=[];
  $('#partitionList').innerHTML='<div class="panel-empty">连接后显示分区</div>';
}

function renderProcesses(processes){
  if(processes) processItems=processes;
  const query=($('#processSearch').value||'').trim().toLowerCase();
  const items=processItems.filter(process=>!query||`${process.PID} ${process.Command} ${process.User} ${process.Ports}`.toLowerCase().includes(query));
  items.sort((left,right)=>{
    const field=processSort.field;
    const a=field==='Command'?left.Command.toLowerCase():Number(left[field]);
    const b=field==='Command'?right.Command.toLowerCase():Number(right[field]);
    return (typeof a==='string'?a.localeCompare(b):a-b)*processSort.direction;
  });
  if(!items.length){
    $('#processList').innerHTML='<div class="panel-empty">暂无进程信息</div>';
    return;
  }
  $('#processList').innerHTML=items.map(process=>`<div class="process-row" data-pid="${process.PID}"><span>${process.PID}</span><span>${process.CPU.toFixed(1)}%</span><span>${process.Memory.toFixed(1)}%</span><span class="process-name" title="${escapeHtml(process.Command)} · ${escapeHtml(process.User)}"><b>${escapeHtml(process.Command)}</b>${process.Ports?`<small>:${escapeHtml(process.Ports)}</small>`:''}</span></div>`).join('');
  $('#processList').querySelectorAll('.process-row').forEach(row=>row.oncontextmenu=event=>openProcessContextMenu(event,items.find(process=>process.PID===Number(row.dataset.pid))));
}

$('#processSearch').addEventListener('input',()=>renderProcesses());
document.querySelectorAll('[data-process-sort]').forEach(button=>button.onclick=()=>{
  const field=button.dataset.processSort;
  processSort={field,direction:processSort.field===field?-processSort.direction:(field==='Command'?1:-1)};
  document.querySelectorAll('[data-process-sort]').forEach(item=>item.classList.toggle('active',item===button));
  renderProcesses();
});

function openProcessContextMenu(event,process){
  if(!process) return;
  event.preventDefault();
  event.stopPropagation();
  const menu=$('#processContextMenu');
  menu.innerHTML=`<div class="process-menu-title"><b>${escapeHtml(process.Command)}</b><span>PID ${process.PID}${process.Ports?' · 端口 '+escapeHtml(process.Ports):''}</span></div><button data-signal="TERM">终止进程</button><button data-signal="KILL" class="danger">强制结束</button><div class="context-separator"></div><button data-signal="STOP">暂停进程</button><button data-signal="CONT">继续进程</button>`;
  menu.hidden=false;
  const width=menu.offsetWidth,height=menu.offsetHeight;
  menu.style.left=`${Math.max(8,Math.min(event.clientX,window.innerWidth-width-8))}px`;
  menu.style.top=`${Math.max(8,Math.min(event.clientY,window.innerHeight-height-8))}px`;
  menu.querySelectorAll('[data-signal]').forEach(button=>button.onclick=async()=>{
    menu.hidden=true;
    const signal=button.dataset.signal;
    if(['TERM','KILL'].includes(signal)&&!confirm(`${signal==='KILL'?'强制结束':'终止'}进程 ${process.Command}（PID ${process.PID}）？`)) return;
    try{await SignalProcess(sessionId,process.PID,signal);setTimeout(loadServerStats,300)}catch(error){alert(String(error))}
  });
}

document.addEventListener('pointerdown',event=>{if(!event.target.closest('#processContextMenu'))$('#processContextMenu').hidden=true});
document.addEventListener('keydown',event=>{if(event.key==='Escape')$('#processContextMenu').hidden=true});

function renderPartitions(partitions){
  if(!partitions?.length){
    $('#partitionList').innerHTML='<div class="panel-empty">暂无分区信息</div>';
    return;
  }
  $('#partitionList').innerHTML=partitions.map(partition=>`<div class="partition-row" title="${escapeHtml(partition.Device)}"><div class="partition-info"><b>${escapeHtml(partition.Mount)}</b><span>${formatBytes(partition.Used)} / ${formatBytes(partition.Total)}</span></div><div class="partition-bar"><i style="width:${Math.max(0,Math.min(100,partition.Percent))}%"></i></div></div>`).join('');
}

function updateResource(id,value,label){
  const bounded=Math.max(0,Math.min(100,Number(value)||0));
  $(`#${id}Value`).textContent=label;
  $(`#${id}Bar`).style.width=`${bounded}%`;
  const item=$(`#${id}Bar`).closest('.resource');
  item.dataset.level=bounded>=85?'high':bounded>=65?'warn':'idle';
}

function percent(used,total){
  return total?used*100/total:0;
}

function formatBytes(value){
  if(!value) return '0 B';
  if(value<1024) return `${value} B`;
  if(value<1048576) return `${(value/1024).toFixed(1)} KB`;
  if(value<1073741824) return `${(value/1048576).toFixed(1)} MB`;
  return `${(value/1073741824).toFixed(1)} GB`;
}

function setStatus(text,color){$('#status').textContent='● '+text;$('#status').style.color=color}
function formatSize(n){return n<1024?n+' B':n<1048576?(n/1024).toFixed(1)+' KB':(n/1048576).toFixed(1)+' MB'}
function escapeHtml(value){const div=document.createElement('div');div.textContent=value;return div.innerHTML}

window.addEventListener('beforeunload',()=>{for(const id of sshSessions.keys()) Disconnect(id)});
async function applyDebug(enabled){
  const button=$('#debugButton');
  button.classList.toggle('active',enabled);
  button.textContent=enabled?'调试日志开':'调试日志';
  localStorage.setItem('nekssh.desktop.debug',enabled?'1':'0');
  try{
    const logPath=await SetDebug(enabled);
    if(enabled) setStatus(`调试日志：${logPath}`,'#e0af68');
  }catch(error){alert(String(error))}
}
$('#debugButton').onclick=()=>applyDebug(!$('#debugButton').classList.contains('active'));
applyDebug(localStorage.getItem('nekssh.desktop.debug')==='1');
renderHosts();
