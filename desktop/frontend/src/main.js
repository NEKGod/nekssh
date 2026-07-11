import './style.css';
import './terminal.css';
import './hosts.css';
import '@xterm/xterm/css/xterm.css';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { Connect, DeletePath, Disconnect, DownloadDirectory, DownloadFile, Input, ListFiles, MakeDirectory, Resize, ScanHostKey, TrustHostKey, UploadFile } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

document.querySelector('#app').innerHTML = `
<div class="shell">
  <aside><h1>NekSSH</h1><button id="newConnection">＋ 新建连接</button><div id="hosts"></div><div class="muted">密码和私钥不会保存</div></aside>
  <main><header><span id="sessionTitle">未连接</span><span id="status">● 就绪</span></header>
    <section id="welcome"><h2>连接 SSH 服务器</h2><p>连接在本机直接建立，不会启动浏览器服务。</p></section>
    <section id="workspace" hidden><div id="terminal"></div><div id="files"><div class="filebar"><b id="path">.</b><span><button id="mkdir">新建目录</button> <button id="upload">上传</button> <button id="refresh">刷新</button></span></div><div id="fileList"></div></div></section>
  </main>
</div>
<dialog id="connectDialog"><form id="connectForm"><h2 id="dialogTitle">新建 SSH 连接</h2>
  <label>名称<input name="name" required placeholder="生产服务器"></label><label>主机<input name="host" required placeholder="192.168.1.10"></label>
  <div class="row"><label>用户名<input name="username" required placeholder="root"></label><label class="port">端口<input name="port" type="number" value="22"></label></div>
  <label>密码<input name="password" type="password"></label><label>私钥<textarea name="privateKey" rows="4"></textarea></label>
  <div class="actions"><button type="button" id="cancel">取消</button><button type="submit" class="primary">保存并连接</button></div>
</form></dialog>`;

const $ = selector => document.querySelector(selector);
const fit = new FitAddon();
const term = new Terminal({cursorBlink:true,scrollback:10000,fontSize:14,fontFamily:'Consolas, Cascadia Mono, monospace',theme:{background:'#080b10',foreground:'#d8dee9',cursor:'#73daca'}});
term.loadAddon(fit); term.open($('#terminal'));
let sessionId = '', currentPath = '.', editingIndex = -1;
let hosts = JSON.parse(localStorage.getItem('nekssh.desktop.hosts') || '[]');

function saveHosts(){ localStorage.setItem('nekssh.desktop.hosts', JSON.stringify(hosts)); renderHosts(); }
function renderHosts(){
  $('#hosts').innerHTML='';
  hosts.forEach((host,index)=>{const row=document.createElement('div');row.className='saved-host';row.innerHTML=`<span><b>${escapeHtml(host.name)}</b><small>${escapeHtml(host.username)}@${escapeHtml(host.host)}:${host.port}</small></span><span><button data-edit="1">✎</button><button data-delete="1">×</button></span>`;row.onclick=event=>{if(event.target.dataset.delete){event.stopPropagation();if(confirm(`删除“${host.name}”？`)){hosts.splice(index,1);saveHosts()}}else openDialog(host,index)};$('#hosts').append(row)});
}
function openDialog(host={}, index=-1){editingIndex=index;const form=$('#connectForm');form.reset();form.name.value=host.name||'';form.host.value=host.host||'';form.username.value=host.username||'';form.port.value=host.port||22;$('#dialogTitle').textContent=index<0?'新建 SSH 连接':`连接 ${host.name}`;$('#connectDialog').showModal();setTimeout(()=>form.password.focus(),50)}

term.onData(data=>sessionId&&Input(sessionId,data));
term.onResize(({cols,rows})=>sessionId&&Resize(sessionId,cols,rows));
new ResizeObserver(()=>fit.fit()).observe($('#terminal'));
$('#newConnection').onclick=()=>openDialog(); $('#cancel').onclick=()=>$('#connectDialog').close();

$('#connectForm').onsubmit=async event=>{
  event.preventDefault(); const data=Object.fromEntries(new FormData(event.target));
  const saved={name:data.name.trim(),host:data.host.trim(),username:data.username.trim(),port:Number(data.port)};
  editingIndex<0?hosts.push(saved):hosts[editingIndex]=saved; saveHosts();
  sessionId='session-'+Date.now(); fit.fit();
  const request={ID:sessionId,Host:saved.host,Username:saved.username,Password:data.password,PrivateKey:data.privateKey,Passphrase:'',Port:saved.port,Cols:term.cols,Rows:term.rows};
  setStatus('正在连接','#e0af68');
  try{await connectWithTrust(request);$('#connectDialog').close();$('#welcome').hidden=true;$('#workspace').hidden=false;$('#sessionTitle').textContent=`${saved.name} · ${saved.username}@${saved.host}`;term.clear();term.focus();fit.fit();await loadFiles('.')}catch(error){sessionId='';setStatus('连接失败','#f7768e');alert(String(error))}
};

async function connectWithTrust(request){try{return await Connect(request)}catch(error){if(!String(error).includes('主机指纹尚未信任'))throw error;const fingerprint=await ScanHostKey(request.Host,request.Port);if(!confirm(`首次连接此服务器。\n\nSHA-256 指纹：\n${fingerprint}\n\n确认信任并继续连接吗？`))throw new Error('用户取消了主机指纹确认');await TrustHostKey(request.Host,request.Port);return Connect(request)}}
EventsOn('ssh:output',(id,data)=>id===sessionId&&term.write(data));
EventsOn('ssh:connected',id=>{if(id===sessionId)setStatus('已连接','#73daca')});
EventsOn('ssh:closed',id=>{if(id===sessionId)setStatus('已断开','#8996aa')});

async function loadFiles(remotePath){try{const entries=await ListFiles(sessionId,remotePath);currentPath=entries[0]?.Path?.replace(/\/[^/]+$/,'')||remotePath;$('#path').textContent=currentPath;const list=$('#fileList');list.innerHTML='<div class="file head"><span>名称</span><span>权限</span><span>用户</span><span>组</span><span>大小</span></div>';for(const file of entries){const row=document.createElement('div');row.className='file';row.innerHTML=`<span>${file.Directory?'📁':'📄'} ${escapeHtml(file.Name)}</span><span>${file.Mode}</span><span>${file.Owner}</span><span>${file.Group}</span><span>${file.Directory?'-':formatSize(file.Size)}</span>`;row.ondblclick=()=>file.Directory?loadFiles(file.Path):DownloadFile(sessionId,file.Path);row.oncontextmenu=event=>fileMenu(event,file);list.append(row)}}catch(error){alert(String(error))}}
$('#refresh').onclick=()=>loadFiles(currentPath);
$('#upload').onclick=async()=>{try{await UploadFile(sessionId,currentPath);await loadFiles(currentPath)}catch(error){alert(String(error))}};
$('#mkdir').onclick=async()=>{const name=prompt('新目录名称');if(!name)return;try{await MakeDirectory(sessionId,currentPath,name);await loadFiles(currentPath)}catch(error){alert(String(error))}};
async function fileMenu(event,file){event.preventDefault();const action=prompt(`${file.Name}\n\n输入操作：download 或 delete`, 'download');if(action==='download'){try{file.Directory?await DownloadDirectory(sessionId,file.Path):await DownloadFile(sessionId,file.Path)}catch(error){alert(String(error))}}if(action==='delete'&&confirm(`确定删除“${file.Name}”吗？${file.Directory?'\n目录内容也会一并删除。':''}`)){try{await DeletePath(sessionId,file.Path);await loadFiles(currentPath)}catch(error){alert(String(error))}}}
function setStatus(text,color){$('#status').textContent='● '+text;$('#status').style.color=color}
function formatSize(n){return n<1024?n+' B':n<1048576?(n/1024).toFixed(1)+' KB':(n/1048576).toFixed(1)+' MB'}
function escapeHtml(value){const div=document.createElement('div');div.textContent=value;return div.innerHTML}
window.addEventListener('beforeunload',()=>sessionId&&Disconnect(sessionId));
renderHosts();
