import './settings-panel.css';
import {defaultSettings,normalizeSettings,terminalFontStack} from './settings-model.mjs';
import {icon} from './ui-icons.mjs';

export function initSettingsPanel({store,onApply}){
  const dialog=document.createElement('dialog');dialog.id='settingsDialog';dialog.setAttribute('aria-labelledby','settingsTitle');
  const select=(name,label,description,options)=>`<label class="settings-row"><span><b>${label}</b><small>${description}</small></span><select name="${name}">${options.map(([value,text])=>`<option value="${value}">${text}</option>`).join('')}</select></label>`;
  const toggle=(name,label,description)=>`<label class="settings-row"><span><b>${label}</b><small>${description}</small></span><span class="settings-switch"><input type="checkbox" name="${name}"><i aria-hidden="true"></i></span></label>`;
  const intervals=[[5,'5 秒'],[10,'10 秒'],[15,'15 秒'],[30,'30 秒'],[60,'60 秒']];
  dialog.innerHTML=`<form id="settingsForm"><div class="settings-header"><div>${icon('settings')}<span><h2 id="settingsTitle">设置</h2><p>调整属于你的工作台</p></span></div><button type="button" data-settings-close aria-label="关闭设置">×</button></div>
    <div class="settings-layout"><nav class="settings-nav" role="tablist" aria-label="设置分类" aria-orientation="vertical">${[['appearance','外观','layers'],['terminal','终端','terminal'],['workspace','工作区','sidebar']].map(([id,label,glyph],i)=>`<button type="button" role="tab" id="settingsTab-${id}" aria-controls="settingsPage-${id}" aria-selected="${i===0}" tabindex="${i===0?0:-1}" data-settings-tab="${id}">${icon(glyph)}${label}</button>`).join('')}<p>设置保存在这台电脑<br>不影响服务器配置</p></nav>
    <div class="settings-content">
      <section id="settingsPage-appearance" role="tabpanel" aria-labelledby="settingsTab-appearance" data-settings-page="appearance"><h3>外观</h3><p class="settings-intro">让界面更适合你的阅读习惯。</p>
        ${select('accent','强调色','用于主要按钮、选中状态和焦点提示。',[['mint','薄荷绿'],['blue','晴空蓝'],['violet','柔雾紫']])}
        ${select('density','界面密度','调整文件、进程列表和工具栏的留白。',[['comfortable','舒适'],['compact','紧凑']])}
        <div class="settings-appearance-preview"><span>界面预览</span><div><span class="settings-preview-tag">已选择</span><button type="button" tabindex="-1">主要操作</button></div><small>保存后应用到工作台，取消不改变当前界面。</small></div>
      </section>
      <section id="settingsPage-terminal" role="tabpanel" aria-labelledby="settingsTab-terminal" data-settings-page="terminal" hidden><h3>终端</h3><p class="settings-intro">字体设置会立即应用到当前终端，无需重连。</p>
        ${select('fontFamily','等宽字体','字体未安装时，自动使用本机等宽字体。',[['Consolas','Consolas'],['Cascadia Mono','Cascadia Mono'],['JetBrains Mono','JetBrains Mono'],['monospace','系统等宽字体']])}
        <label class="settings-row"><span><b>字号</b><small>11–24 px，建议从 14 px 开始。</small></span><input name="fontSize" type="number" min="11" max="24" step="1" required aria-label="终端字号"></label>
        ${select('lineHeight','行距','较大的行距更易读，但可显示的行数会减少。',[[1,'标准 · 1.0'],[1.2,'宽松 · 1.2'],[1.4,'加宽 · 1.4']])}
        ${select('cursorStyle','光标样式','终端程序可能自行设置光标外观。',[['block','方块'],['bar','竖线'],['underline','下划线']])}
        ${toggle('cursorBlink','光标闪烁','在终端空闲时显示闪烁光标。')}
        <label class="settings-row"><span><b>历史回滚行数</b><small>1,000–100,000 行；减少会丢弃较早的终端输出。</small></span><input name="scrollback" type="number" min="1000" max="100000" step="1000" required aria-label="历史回滚行数"></label>
        <div class="settings-terminal-preview" aria-label="终端字体预览"><span>root@server</span>:~/workspace$<br>const message = "你好，NekSSH";<br><span>✓</span> Ready to build<span class="settings-preview-cursor"></span></div>
      </section>
      <section id="settingsPage-workspace" role="tabpanel" aria-labelledby="settingsTab-workspace" data-settings-page="workspace" hidden><h3>工作区</h3><p class="settings-intro">保留常用信息，减少不必要的干扰。</p>
        ${toggle('showSidebar','显示资源侧栏','显示 CPU、内存、进程和磁盘；也可用工具栏切换。')}
        ${select('statsInterval','资源刷新频率','资源、进程和分区并发刷新；较低频率可减少请求。',intervals)}
        ${select('portsInterval','端口刷新频率','仅在打开“端口与穿透”面板时自动刷新。',intervals)}
        ${toggle('debug','调试日志','在客户端记录诊断信息，可从底部状态栏切换。')}
        <p class="settings-note">恢复默认只重置本页偏好，不会删除 SSH 连接、密码、书签或端口使用记录。</p>
      </section>
    </div></div><div id="settingsError" role="alert" hidden></div><div class="settings-footer"><button type="button" id="settingsReset">恢复默认</button><span></span><button type="button" data-settings-close>取消</button><button type="submit" class="primary">保存并应用</button></div></form>`;
  document.body.append(dialog);
  const form=dialog.querySelector('form'),$=s=>dialog.querySelector(s);
  form.noValidate=true;
  const accents={mint:'#79d7be',blue:'#83b8f3',violet:'#b8a0f0'};
  function draft(){
    const data=Object.fromEntries(new FormData(form));
    for(const name of ['fontSize','lineHeight','scrollback','statsInterval','portsInterval']) data[name]=Number(data[name]);
    for(const name of ['cursorBlink','showSidebar','debug']) data[name]=form.elements.namedItem(name).checked;
    return normalizeSettings(data);
  }
  function preview(){
    const value=draft();dialog.style.setProperty('--settings-accent',accents[value.accent]);
    const terminal=$('.settings-terminal-preview');
    terminal.style.fontFamily=terminalFontStack(value.fontFamily);
    terminal.style.fontSize=value.fontSize+'px';terminal.style.lineHeight=String(value.lineHeight);
    terminal.dataset.cursor=value.cursorStyle;terminal.classList.toggle('blink',value.cursorBlink);
    $('.settings-appearance-preview').dataset.density=value.density;
  }
  function fill(value){
    for(const [key,item] of Object.entries(value)){
      const control=form.elements.namedItem(key);if(!control) continue;
      if(control.type==='checkbox') control.checked=item;else control.value=String(item);
    }
    preview();$('#settingsError').hidden=true;
  }
  function tab(id,focus=false){
    dialog.querySelectorAll('[data-settings-tab]').forEach(button=>{
      const active=button.dataset.settingsTab===id;button.setAttribute('aria-selected',String(active));button.tabIndex=active?0:-1;if(active&&focus)button.focus();
    });
    dialog.querySelectorAll('[data-settings-page]').forEach(page=>page.hidden=page.dataset.settingsPage!==id);
    $('.settings-content').scrollTop=0;
  }
  dialog.querySelectorAll('[data-settings-tab]').forEach(button=>button.onclick=()=>tab(button.dataset.settingsTab));
  $('.settings-nav').addEventListener('keydown',event=>{
    const tabs=[...dialog.querySelectorAll('[data-settings-tab]')],index=tabs.indexOf(document.activeElement);
    if(index<0||!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return;
    event.preventDefault();const next=event.key==='Home'?0:event.key==='End'?tabs.length-1:(index+(event.key==='ArrowDown'?1:-1)+tabs.length)%tabs.length;tab(tabs[next].dataset.settingsTab,true);
  });
  form.addEventListener('input',preview);form.addEventListener('change',preview);
  function close(){dialog.close()}
  dialog.querySelectorAll('[data-settings-close]').forEach(button=>button.onclick=close);
  $('#settingsReset').onclick=()=>fill(defaultSettings);
  form.onsubmit=async event=>{
    event.preventDefault();
    const invalid=[...form.elements].find(control=>control.willValidate&&!control.validity.valid);
    if(invalid){tab(invalid.closest('[data-settings-page]').dataset.settingsPage);invalid.reportValidity();return}
    const submit=form.querySelector('[type=submit]');submit.disabled=true;
    try{const value=store.save(draft());await onApply(value);close()}
    catch(error){$('#settingsError').textContent=String(error);$('#settingsError').hidden=false}
    finally{submit.disabled=false}
  };
  return {open(){fill(store.get());tab('appearance');dialog.showModal()},close};
}
