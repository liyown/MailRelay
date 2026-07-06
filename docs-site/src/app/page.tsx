import Link from 'next/link';

const scenarios = [
  { n: '01', title: '手机就是运维终端', copy: '在地铁、机场或只有邮件客户端的设备上，发送 deploy、backup 或 status。无需 VPN、SSH 客户端或另一个专用 App。', subject: 'deploy', result: 'workflow · 3 steps · success' },
  { n: '02', title: '让旧设备加入自动化', copy: '打印机、NAS、功能机、告警系统和受限企业终端只要能发邮件，就能调用声明过的 Command。', subject: 'archive-report', result: 'queue · accepted · #1842' },
  { n: '03', title: '把异步任务交给收件箱', copy: '长任务进入 SQLite Queue。进程重启后恢复 lease，失败有限重试，耗尽后保留在 dead letter 等待人工重放。', subject: 'backup photos', result: 'queued · retry 1/4' },
  { n: '04', title: '邮件直达 Agent 与 MCP', copy: '把会议纪要、附件或结构化参数交给固定模型和 allowlist Tool。邮件提供输入，但不能改写 Endpoint、系统提示或工具权限。', subject: 'summary', result: 'agent → smtp reply' },
];

const emailAdvantages = [
  ['Universal', '每台手机、电脑和大量设备都已有邮件客户端。'],
  ['Store & forward', '离线不是失败；邮件会等待网络恢复后继续投递。'],
  ['Rich payload', 'Subject、正文、Header、附件和线程天然承载命令上下文。'],
  ['Human audit', '请求和回复保留在人类可读的会话中，SQLite 同步记录机器审计。'],
];

const comparisons = [
  ['SSH', '实时、强大', '依赖网络入口、密钥和客户端；不适合低频异步触发'],
  ['Chat bot', '交互自然', '需要平台账号、Bot 接入和持续在线的第三方通道'],
  ['Raw webhook', '机器调用简单', '人和旧设备不容易调用；认证、重试和目录通常要重写'],
  ['Custom dashboard', '体验可完全定制', '需要设计、部署、登录、移动适配和长期维护'],
  ['MailRelay', '任何邮件客户端可用', '专注声明式、低频、可审计的单机自动化'],
];

const trust = [
  ['Two keys, one door', '发件人 allowlist 与常量时间 Token 校验必须同时通过。'],
  ['Exactly once at the edge', 'Message-ID 与 SQLite 原子 claim 阻止重复邮件再次执行。'],
  ['Execution is not delivery', 'Handler 结果先落 SQLite Outbox；SMTP 重试不会重跑命令。'],
  ['The network has boundaries', 'HTTP 仅允许固定 HTTPS 主机，并检查 DNS、IP 和跨主机重定向。'],
  ['Failure stays visible', 'Queue 与 Reply 耗尽重试后进入 dead letter，只能显式 replay。'],
  ['Secrets leave no trail', 'Token、密码、API Key、完整正文和 sensitive 参数不会进入审计。'],
];

const handlers = [
  ['stable', 'HTTP & Webhook', '固定 HTTPS 目标与签名事件，适合第一个生产命令。'],
  ['beta', 'Workflow & Queue', '组合声明式步骤，或通过 SQLite 异步执行与恢复。'],
  ['experimental', 'Plugin & Shell', '运行配置固定的绝对路径，不经过 shell 解释器。'],
  ['experimental', 'Agent & MCP', '固定模型、提示和 allowlist Tool，邮件只能提供已声明参数。'],
];

export default function HomePage() {
  return (
    <div className="landing">
      <Header />
      <main>
        <section className="hero">
          <div>
            <span className="eyebrow"><b>NEW</b> single-host automation</span>
            <h1>Email is the command line for <em>everything</em> you run.</h1>
            <p className="lead">MailRelay 把经过认证的邮件转换成声明式 Command，再交给安全、可组合的 Handler。任何能发邮件的设备，都已经是你的远程控制终端。</p>
            <Install />
            <div className="hero-actions"><Link href="/docs">阅读文档</Link><a href="#scenarios">查看黄金场景 ↓</a></div>
          </div>
          <Terminal />
        </section>

        <section className="manifesto landing-section">
          <div className="manifesto-inner">
            <p className="section-label">The big promise</p>
            <h2>The universal remote for everything you run.</h2>
            <p>Zero apps. Zero dashboards. Just send an email.</p>
            <div className="manifesto-proof"><span>One inbox.</span><span>Every declared command.</span><span>A reply for every result.</span></div>
          </div>
        </section>

        <section className="landing-section" id="scenarios"><div className="section-inner">
          <p className="section-label">Golden scenarios</p>
          <h2>Automation that starts where you already are.</h2>
          <p className="section-intro">MailRelay 不是高频交互界面。它擅长的是那些重要、低频、异步，而且必须留下记录的远程动作。</p>
          <div className="scenario-list">{scenarios.map((item, i) => <article className={`scenario ${i % 2 ? 'reverse' : ''}`} key={item.n}>
            <div className="scenario-copy"><span>{item.n}</span><h3>{item.title}</h3><p>{item.copy}</p></div>
            <EmailCard subject={item.subject} result={item.result} />
          </article>)}</div>
        </div></section>

        <section className="landing-section why-email"><div className="section-inner">
          <p className="section-label">Why email wins</p><h2>The oldest universal client is still the most available.</h2>
          <p className="section-intro">Email 已经解决身份、寻址、离线投递、富载荷和跨设备访问。MailRelay 不重新发明控制通道，而是在它上面补上严格的 Command Protocol。</p>
          <div className="advantage-grid">{emailAdvantages.map(([title, copy], i) => <article key={title}><span>0{i+1}</span><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div></section>

        <section className="landing-section"><div className="section-inner discovery-layout">
          <div><p className="section-label">Discovery is the interface</p><h2>The command manual writes itself.</h2><p className="section-intro">新增命令只改 YAML。名称、描述、参数、必填标记和示例会自动进入 Catalog；`help` 与 `help deploy` 始终反映正在运行的配置。</p><p className="proof-note">Catalog Hash 变化时，MailRelay 还能告诉你 Added、Removed 和 Updated。</p></div>
          <div className="mail-stack"><MailPreview subject="help" lines={['Available Commands', 'deploy   部署项目', 'backup   备份数据', 'summary  总结附件']} /><MailPreview subject="help deploy" lines={['deploy', 'Description  部署项目', 'Parameters   env* · version', 'Example      env=prod']} /></div>
        </div></section>

        <section className="landing-section"><div className="section-inner">
          <p className="section-label">The protocol</p><h2>From inbox to execution, every boundary stays small.</h2>
          <p className="section-intro">Receiver 不知道 HTTP，Router 不知道邮件，Handler 不知道 IMAP。每层只做一件事，因此扩展能力不需要改写控制平面。</p>
          <div className="protocol">{[['01','Mail','接收'],['02','Parser','规范化'],['03','Auth','认证去重'],['04','Router','解析命令'],['05','Handler','安全执行'],['06','Reply','审计回复']].map(([n,t,d]) => <div className="protocol-item" key={n}><span>{n}</span><strong>{t}</strong><small>{d}</small></div>)}</div>
        </div></section>

        <section className="landing-section comparison"><div className="section-inner">
          <p className="section-label">MailRelay vs. the usual remote controls</p><h2>Not another control panel to keep alive.</h2>
          <p className="section-intro">MailRelay 不替代持续交互的终端或专业运维平台。它消灭的是“为了几个远程动作，再维护一整套入口”的成本。</p>
          <div className="comparison-table"><div className="comparison-head"><span>方式</span><span>擅长</span><span>需要承担</span></div>{comparisons.map((row) => <div className={row[0] === 'MailRelay' ? 'featured' : ''} key={row[0]}>{row.map(cell => <span key={cell}>{cell}</span>)}</div>)}</div>
        </div></section>

        <section className="landing-section trust-section" id="security"><div className="section-inner">
          <p className="section-label">Trust is a runtime feature</p><p className="trust-kicker">Built to be safe by default.</p><h2>“安全”不是一句标语，是每次执行的路径。</h2>
          <p className="section-intro">危险能力默认关闭。认证、幂等、网络边界、审计、回复重试和死信恢复都在运行时内建，而不是留给每个 Handler 自己实现。</p>
          <div className="trust-grid">{trust.map(([title, copy], i) => <article key={title}><span>0{i+1}</span><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div></section>

        <section className="landing-section" id="handlers"><div className="section-inner">
          <p className="section-label">One router, every horizon</p><h2>Start boring. Grow into anything.</h2>
          <p className="section-intro">先用 Stable HTTP/Webhook 跑通黄金路径；再按风险需要引入 Workflow、Queue、Plugin、Shell、Agent 和 MCP。成熟度直接进入 Discovery，不隐藏边界。</p>
          <div className="stories">{handlers.map(([tag,title,text]) => <article className="story" key={title}><span className={`maturity ${tag}`}>{tag}</span><h3>{title}</h3><p>{text}</p></article>)}</div>
        </div></section>

        <section className="landing-section final-cta"><div className="section-inner cta">
          <p className="section-label">Start with one command</p><h2>Five minutes from inbox to automation.</h2>
          <p className="section-intro">不要先造平台。声明一个固定 HTTPS Command，给自己发一封邮件，然后让真实需求决定第二个 Handler。</p>
          <div className="quickstart"><div><span>01</span><code>mailrelay init</code></div><div><span>02</span><code>mailrelay doctor</code></div><div><span>03</span><code>mailrelay run</code></div></div>
          <div className="cta-actions"><Link className="primary-button" href="/docs/getting-started/installation">开始使用</Link><Link className="secondary-button" href="/docs/getting-started/first-command">创建第一个命令</Link></div>
        </div></section>
      </main>
      <footer className="landing-footer"><span>mailrelay. · becomeopc</span><span>Email → Command → Execution → Response</span></footer>
    </div>
  );
}

function Header() { return <header className="site-header"><nav className="landing-nav"><Link className="brand" href="/">mailrelay.</Link><div className="nav-links"><a href="#scenarios">场景</a><a href="#security">安全</a><a href="#handlers">Handlers</a><Link href="/docs">文档</Link></div><span className="nav-spacer"/><a className="github-link" href="https://github.com/liyown/MailRelay">GitHub</a></nav></header>; }
function Install() { return <div className="install"><div className="install-tabs"><span className="active">Go</span><span>macOS</span><span>Linux</span></div><div className="install-command"><span>$</span><code>go install github.com/becomeopc/opc-mailrelay/cmd/mailrelay@latest</code></div></div>; }
function Terminal() { return <div className="terminal"><div className="window-dots"><i/><i/><i/></div><pre><b>Subject</b>   deploy{`\n`}<b>From</b>      ops@example.com{`\n`}<b>Token</b>     verified{`\n\n`}<span># parser</span>     mail → CommandRequest{`\n`}<span># router</span>     deploy → workflow{`\n`}<span># handler</span>    3 steps completed{`\n`}<span># audit</span>      saved to SQLite{`\n`}<span># response</span>   SMTP reply queued</pre></div>; }
function EmailCard({ subject, result }: { subject: string; result: string }) { return <div className="email-card"><div className="email-meta"><span>From</span><b>me@example.com</b><span>Subject</span><b>{subject}</b></div><div className="email-body"><span>message authenticated</span><strong>{result}</strong><small>Reply queued · audit saved</small></div></div>; }
function MailPreview({ subject, lines }: { subject: string; lines: string[] }) { return <div className="mail-preview"><div><span>Subject</span><strong>{subject}</strong></div><pre>{lines.join('\n')}</pre></div>; }
