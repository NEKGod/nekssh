const clientBridge=`你现在位于 NekSSH 连接的 SSH 服务器上，客户机是运行 NekSSH 的 Windows 电脑，不是当前服务器。服务器上的 http://127.0.0.1:18765 通过 SSH 反向通道连接客户机。先用带连接和总时限的 HTTP 请求读取该地址的 / 和 /docs；只有 / 返回 service=nekssh 且 ok=true 才继续，实际接口以 /docs 为准。连接失败或身份不符就报告具体错误，让用户检查 NekSSH 的 SSH 连接和反射状态，不要扫描其他端口，也不要改在服务器上运行来冒充客户机。`;
const boundaries=`以下是能力说明，不是对客户机任意操作的授权。只执行用户在“我的任务”中指定的事情；任务未填写则等待用户补充。不要安装依赖、修改安全配置或操作无关文件来强行完成；缺少必要条件时先说明。不要把本机地址描述成公网地址。`;

export const terminalSkills=Object.freeze([
  {id:'client-browser',label:'控制客户机 Chrome',description:'让服务器上的 AI 通过 NekSSH 操作客户机浏览器',prompt:`【NekSSH 技能：控制客户机 Chrome】
${clientBridge}
这里没有现成的 /browser 接口。使用现有 POST /v1/run 在客户机启动独立 Chrome 调试实例，并把调试端口通过 SSH 反射到当前服务器，再通过 Chrome DevTools Protocol（CDP）操作，不要声称仅打开浏览器就已经可以点击或截图。
具体流程：先 GET /v1/jobs 检查是否有本次任务可复用的浏览器任务，核实它返回的 remote 地址的 /json/version。需要新实例时，通过 /v1/run 执行 Windows 命令查找 chrome.exe，结果读取 GET /v1/jobs/{id}/log；不能使用服务器上的 Chrome 路径。找到后用独立的 workspace 下浏览器目录启动，传入 --user-data-dir=<独立绝对路径>、--remote-debugging-address=127.0.0.1、--remote-debugging-port=<调试端口>、--no-first-run 和 about:blank。不要复用默认用户目录，不要关闭日常 Chrome，不要使用 --no-sandbox 或监听 0.0.0.0。
请求 /v1/run 的 JSON 包含 command（客户机 cmd.exe /c 命令）和 port（Chrome 的调试端口，例如 9222）。JSON 用脚本或文件正确编码，不要手拼多层 shell 引号。只有返回 ok=true、无 reflectError 且含 remote 才继续。记住 id、local、remote。remote 是服务器侧地址列表，从返回值选择实际地址，不要假定客户机能被直接访问。短时间有界重试该地址的 /json/version，确认返回 webSocketDebuggerUrl；端口冲突或启动失败时读取任务日志并报告，不要连接未知的已有调试服务。
若服务器已有 Playwright，可用 chromium.connect_over_cdp 连接这个反射地址；也可使用已有 CDP WebSocket 工具。先列出页面确认身份，再根据用户任务打开页面、读取页面、点击、输入或截图。不要在服务器另起一个本地浏览器代替客户机浏览器。缺少客户端库时先说明并征求安装意愿。截图通过 CDP 获取，说明保存在服务器还是客户机。独立浏览器没有日常浏览器登录状态，需要登录时让用户在客户机窗口手动完成；不要读取默认 Chrome 的 Cookie 或密码。
任务启动成功必须以实际 CDP 连接和页面结果为证据。停止时仅处理本任务记录的 /v1/jobs/{id}/stop，不要按进程名杀死全部 Chrome。涉及提交、支付、删除等操作时在最终确认前停下。
${boundaries}
我的任务：`},
  {id:'client-run',label:'在客户机运行项目',description:'把服务器项目放到 Windows 客户机运行，并反射服务端口',prompt:`【NekSSH 技能：在客户机运行项目】
${clientBridge}
需要的文件通过 POST /v1/files 写入客户机 workspace，JSON 为 {"files":[{"path":"相对路径","content":"文本"}]}；二进制使用 contentBase64，路径用 / 分隔，不能包含 ..。仅传输任务需要的文件，不传输无关凭据。
通过 POST /v1/run 发送 {"command":"客户机启动命令","port":3000}。命令在客户机 Windows 的 workspace 中由 cmd.exe /c 执行，不是 Linux shell；服务应绑定 127.0.0.1，PORT 环境变量会设置为 port。不需要服务端口时省略 port。不要假定客户机安装了与服务器相同的运行时。
检查响应 ok、id、pid、local、remote、reflectError。remote 为服务器侧访问地址列表；只有反射成功且从服务器访问实际 remote 确认服务响应后，才报告可用。通过 GET /v1/jobs、GET /v1/jobs/{id}/log 查看状态和日志，POST /v1/jobs/{id}/stop 停止该任务的进程树。操作失败报告日志，不要无限重试或启动重复任务。
${boundaries}
我的任务：`},
  {id:'control-guide',label:'双向控制与通道排查',description:'解释控制方向并检查服务器到客户机的可用通道',prompt:`【NekSSH 技能：双向控制与通道排查】
${clientBridge}
区分两个方向：① 当前 SSH 服务器访问 127.0.0.1:18765，是控制 Windows 客户机，可写 workspace 文件、运行任务，并把客户机服务反射回服务器；② Windows 客户机访问 127.0.0.1:18766，是通过 NekSSH 控制 SSH 服务器，其 /docs 描述 /v1/run、/v1/files 和 /v1/forward。18766 位于客户机，不要从当前服务器访问同名 localhost 地址来判断客户机控制端是否在线。
先根据用户任务确定方向。服务器控制客户机时实际检测 18765 的身份、/docs 和 /v1/jobs，报告哪一步失败；客户机控制服务器时说明需要在客户机执行的检测，不要伪造在客户机执行过的结果。有多个 SSH 会话时，客户机 18766 调用必须使用实际会话 ID 的 ?session= 参数，不要猜 ID 或随便选择目标服务器。服务端口的 local/remote 必须标明分别位于哪一侧；保持端口只绑定回环地址。
${boundaries}
我的任务：`}
]);

// One logical line prevents newline-based submission even without bracketed paste.
export function skillInput(skill){
  return skill.prompt.replace(/[\r\n\t]+/g,' ').replace(/[\x00-\x1f\x7f-\x9f]/g,'').trimEnd()+' ';
}

export function skillDraft(id,text='',previousID=''){
  const previous=terminalSkills.find(item=>item.id===previousID);
  const task=previous&&text.startsWith(previous.prompt)?text.slice(previous.prompt.length).trimStart():text;
  const skill=terminalSkills.find(item=>item.id===id);
  return skill?skill.prompt+(task?'\n'+task:''):task;
}
