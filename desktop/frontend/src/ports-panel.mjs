import './ports-panel.css';
import {ListeningPorts,StartPortTunnel,StopPortTunnel,PortTunnels} from '../wailsjs/go/main/App.js';
import {ClipboardSetText,BrowserOpenURL} from '../wailsjs/runtime/runtime.js';
import {groupListeningPorts,tcpPortChoices,portTargetKey as key} from './ports-model.mjs';
import {createPortHistory} from './ports-history.mjs';

export function initPortPanel({getSession,getName,getServerKey=getName,getRefreshInterval=()=>10000}){
  let storage;try{storage=window.localStorage}catch{}
  const history=createPortHistory(storage);
  const dialog=document.createElement('dialog');
  dialog.id='portsDialog';
  dialog.innerHTML=`<div class="ports-window">
    <div class="permission-head"><div><b>端口与穿透</b><span id="portsServer"></span></div><button type="button" class="icon-button" data-close aria-label="关闭">×</button></div>
    <div class="ports-toolbar"><input id="portsSearch" placeholder="搜索端口、进程、PID、地址或用户" aria-label="搜索监听端口"><button type="button" id="portsRefresh">刷新</button><span id="portsCount"></span></div>
    <p class="ports-help">Listen 本地转发与一键穿透统一在此管理。保存的规则连接后自动启用；一键穿透默认仅本机可访问。部分进程信息需要服务器 root 权限。</p>
    <div id="portsMessage" role="status" aria-live="polite"></div>
    <section class="ports-tunnels"><div class="ports-tunnels-head"><h3>本地转发 / 穿透</h3><label>排序 <select id="tunnelSort" aria-label="转发卡片排序"><option value="frequent">点击次数</option><option value="recent">最近点击</option><option value="newest">最新添加</option><option value="port">端口顺序</option></select></label></div><div id="portTunnelList"></div></section>
    <div class="ports-table-wrap"><table class="ports-table"><thead><tr><th>协议</th><th>监听地址</th><th>端口</th><th>PID</th><th>进程 / 用户</th><th>操作</th></tr></thead><tbody id="listeningPorts"></tbody></table></div>
  </div>`;
  document.body.append(dialog);
  const $=selector=>dialog.querySelector(selector);
  const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  let session='',serverKey='',ports=[],tunnels=[],revision=0,tunnelRevision=0,timer=0,loading=null;
  $('#tunnelSort').value=history.mode;
  $('#tunnelSort').onchange=()=>{history.setMode($('#tunnelSort').value);renderTunnels()};
  const busy=new Set();
  const selectedTargets=new Map();
  const valid=(id,version)=>dialog.open&&session===id&&getSession()===id&&revision===version;
  function message(text,error=false){$('#portsMessage').textContent=text;$('#portsMessage').classList.toggle('error',error)}
  function renderPorts(){
    const query=$('#portsSearch').value.trim().toLowerCase();
    const groups=groupListeningPorts(ports);
    const visible=groups.filter(group=>group.Entries.some(({port})=>[port.Protocol,port.Address,port.Port,port.PID,port.Process,port.User,port.Command].join(' ').toLowerCase().includes(query)));
    $('#portsCount').textContent=`${visible.length} / ${groups.length} 个端口`;
    const unique=(group,field)=>[...new Set(group.Entries.map(({port})=>port[field]).filter(Boolean))];
    $('#listeningPorts').innerHTML=visible.length?visible.map(group=>{
      const choices=tcpPortChoices(group);
      const selected=choices.find(({port})=>key(port)===selectedTargets.get(group.Port))||choices.find(({port})=>tunnels.some(t=>t.RemoteAddress===key(port)))||choices[0];
      const active=selected&&tunnels.some(t=>t.RemoteAddress===key(selected.port));
      const pending=selected&&busy.has(key(selected.port));
      const disabled=!selected||active||pending;
      const addresses=unique(group,'Address').join(', ');
      const pids=unique(group,'PID').join(', ')||'—';
      const processes=unique(group,'Process').join(', ')||'未知进程';
      const users=unique(group,'User').join(', ')||'未知用户';
      const details=group.Entries.map(({port})=>`${port.Protocol.toUpperCase()} ${port.Address}:${port.Port} · PID ${port.PID||'—'} · ${port.User||'未知用户'} · ${port.Command||port.Process||'未知进程'}`).join('\n');
      const processCell=group.Entries.length>1?`<details class="port-details"><summary title="展开全部监听详情">${escape(processes)} <small>${group.Entries.length} 项监听</small></summary><div>${group.Entries.map(({port})=>`<p><b>${escape(port.Protocol.toUpperCase())} ${escape(port.Address)} · PID ${port.PID||'—'}</b><span>${escape(port.Process||'未知进程')} / ${escape(port.User||'未知用户')}</span><span>${escape(port.Command)}</span></p>`).join('')}</div></details>`:`<b>${escape(processes)}</b><small>${escape(users)}</small>`;
      const targetSelect=choices.length>1?`<select data-target="${group.Port}" aria-label="端口 ${group.Port} 的 TCP 穿透地址">${choices.map(choice=>`<option value="${choice.index}" ${choice===selected?'selected':''}>${escape(choice.port.Address)}</option>`).join('')}</select>`:'';
      return `<tr><td><span class="port-protocol">${escape(unique(group,'Protocol').join(' / ').toUpperCase())}</span></td><td class="port-address" title="${escape(addresses)}"><span>${escape(addresses)}</span></td><td class="port-number">${group.Port}</td><td class="port-pids" title="${escape(pids)}"><span>${escape(pids)}</span></td><td class="port-process" title="${escape(details)}">${processCell}</td><td><div class="port-group-actions">${targetSelect}<button type="button" data-start="${selected?.index??-1}" ${disabled?'disabled':''}>${!selected?'仅支持 TCP':pending?'建立中…':active?'已穿透':'一键穿透'}</button></div></td></tr>`;
    }).join(''):'<tr><td colspan="6" class="panel-empty">没有匹配的监听端口</td></tr>';
  }
  function renderTunnels(){
    const sorted=history.sort(serverKey,tunnels);
    $('#portTunnelList').innerHTML=sorted.length?sorted.map(t=>{
      // Actions reference the original list, not the sorted card position.
      const index=tunnels.indexOf(t),stats=history.stats(serverKey,t);
      return `<article class="port-tunnel" aria-label="转发 ${escape(t.LocalAddress)} 到 ${escape(t.RemoteAddress)}"><div class="port-tunnel-head"><span class="port-protocol">TCP 转发 · 打开 ${stats.opens} 次</span><span class="port-tunnel-status"><i aria-hidden="true"></i>监听中</span></div><div class="port-tunnel-route"><div><small>本机访问地址</small><b>${escape(t.LocalAddress)}</b></div><span class="port-tunnel-arrow" aria-hidden="true">↓</span><div><small>服务器目标</small><b>${escape(t.RemoteAddress)}</b></div></div><div class="port-tunnel-actions"><button type="button" data-copy="${index}">复制地址</button><button type="button" data-open="${index}" title="仅适用于 HTTP 服务；记录打开次数用于排序">HTTP 打开</button><button type="button" data-stop="${index}">停止</button></div></article>`;
    }).join(''):'<div class="ports-empty-tunnels">尚无穿透，从下方监听端口选择“一键穿透”即可。</div>';
    renderPorts();
  }
  async function refresh(){
    const id=session,version=revision;
    const tunnelVersion=tunnelRevision;
    if(!id||loading===version) return;
    loading=version;$('#portsRefresh').disabled=true;
    try{
      await Promise.allSettled([
        ListeningPorts(id).then(items=>{if(valid(id,version)){ports=items||[];renderPorts()}},error=>{if(valid(id,version))message(`读取监听端口失败：${error}`,true)}),
        PortTunnels(id).then(items=>{if(valid(id,version)&&tunnelVersion===tunnelRevision){tunnels=items||[];renderTunnels()}},error=>{if(valid(id,version))message(`读取穿透状态失败：${error}`,true)})
      ]);
    }finally{if(valid(id,version)){$('#portsRefresh').disabled=false;loading=null}}
  }
  function close(){
    revision++;session='';loading=null;clearInterval(timer);timer=0;
    if(dialog.open) dialog.close();
  }
  function open(){
    const id=getSession();if(!id) return;
    close();session=id;serverKey=getServerKey();ports=[];tunnels=[];busy.clear();selectedTargets.clear();$('#portsSearch').value='';
    $('#portsServer').textContent=getName();message('');renderTunnels();
    $('#listeningPorts').innerHTML='<tr><td colspan="6" class="panel-empty">正在读取服务器监听端口…</td></tr>';
    dialog.showModal();refresh();reschedule();
  }
  function reschedule(){clearInterval(timer);timer=dialog.open?setInterval(refresh,getRefreshInterval()):0}
  dialog.addEventListener('cancel',event=>{event.preventDefault();close()});
  $('#portsSearch').addEventListener('input',renderPorts);
  dialog.addEventListener('change',event=>{
    if(!event.target.matches('select[data-target]')) return;
    const port=ports[Number(event.target.value)];
    if(port){selectedTargets.set(port.Port,key(port));renderPorts()}
  });
  $('#portsRefresh').onclick=()=>{message('');refresh()};
  dialog.addEventListener('click',async event=>{
    const button=event.target.closest('button');if(!button) return;
    if(button.hasAttribute('data-close')){close();return}
    if(!button.matches('[data-start],[data-stop],[data-copy],[data-open]')) return;
    const id=session,version=revision;
    if(!valid(id,version)) return;
    button.disabled=true;
    let pendingKey='';
    try{
      if(button.hasAttribute('data-start')){
        const port=ports[Number(button.dataset.start)];if(!port) return;
        tunnelRevision++;
        pendingKey=key(port);busy.add(pendingKey);renderPorts();
        const tunnel=await StartPortTunnel(id,port.Protocol,port.Address,port.Port);
        if(valid(id,version)){
          tunnelRevision++;
          tunnels=tunnels.filter(item=>item.ID!==tunnel.ID);tunnels.push(tunnel);renderTunnels();
          message(`穿透已建立，本机访问地址：${tunnel.LocalAddress}`);
        }
      }else if(button.hasAttribute('data-stop')){
        const tunnel=tunnels[Number(button.dataset.stop)];if(!tunnel) return;
        tunnelRevision++;
        await StopPortTunnel(id,tunnel.ID);
        if(valid(id,version)){tunnelRevision++;tunnels=tunnels.filter(item=>item.ID!==tunnel.ID);renderTunnels();message('穿透已停止')}
      }else if(button.hasAttribute('data-copy')){
        const tunnel=tunnels[Number(button.dataset.copy)];if(!tunnel) return;
        await ClipboardSetText(tunnel.LocalAddress);if(valid(id,version))message('本机访问地址已复制');
      }else if(button.hasAttribute('data-open')){
        const tunnel=tunnels[Number(button.dataset.open)],scope=serverKey;
        if(tunnel){
          await BrowserOpenURL(`http://${tunnel.LocalAddress}`);
          history.opened(scope,tunnel);
          if(valid(id,version)) renderTunnels();
        }
      }
    }catch(error){if(valid(id,version))message(String(error),true)}
    finally{if(valid(id,version)){if(pendingKey){busy.delete(pendingKey);renderPorts()}button.disabled=false}}
  });
  document.querySelector('#portsButton').onclick=open;
  return {close,reschedule};
}
