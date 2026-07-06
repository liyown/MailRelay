import Link from 'next/link';

const scenarios = [
  { n: '01', title: '移动运维', copy: '通过手机发送 deploy、backup 或 status，无需开放 SSH 入口。', subject: 'deploy', result: 'workflow · 3 steps · success' },
  { n: '02', title: '连接受限设备', copy: 'NAS、打印机与企业终端只需具备邮件发送能力。', subject: 'archive-report', result: 'queue · accepted · #1842' },
  { n: '03', title: '可靠异步任务', copy: 'SQLite Queue 提供恢复、有限重试与 dead letter 重放。', subject: 'backup photos', result: 'queued · retry 1/4' },
  { n: '04', title: '调用 Agent 与 MCP', copy: '模型、提示词与 Tool allowlist 固定；邮件仅提供声明参数。', subject: 'summary', result: 'agent → smtp reply' },
];

const emailAdvantages = [
  ['Universal', '跨设备可用，无需安装专用客户端。'],
  ['Store & forward', '网络恢复后继续投递，适合异步操作。'],
  ['Rich payload', 'Subject、正文、Header 与附件承载完整上下文。'],
  ['Human audit', '邮件会话与 SQLite 审计相互对应。'],
];

const comparisons = [
  ['SSH', '实时控制', '网络入口、密钥与客户端'],
  ['Chat bot', '即时交互', '平台账号、Bot 接入与在线通道'],
  ['Raw webhook', '机器调用', '需自行实现认证、重试与目录'],
  ['Custom dashboard', '定制体验', '持续开发、部署与维护'],
  ['MailRelay', '异步远程操作', '声明式、可审计的单机自动化'],
];

const trust = [
  ['Two keys, one door', '发件人 allowlist 与常量时间 Token 校验同时生效。'],
  ['Exactly once at the edge', 'Message-ID 与 SQLite 原子 claim 阻止重复执行。'],
  ['Execution is not delivery', 'Outbox 将执行结果与 SMTP 投递解耦。'],
  ['The network has boundaries', 'HTTP 固定 HTTPS 主机，并校验 DNS、IP 与重定向。'],
  ['Failure stays visible', '重试耗尽后进入 dead letter，等待显式 replay。'],
  ['Secrets leave no trail', '凭证、完整正文与敏感参数不写入审计。'],
];

const handlers = [
  ['stable', 'HTTP & Webhook', '固定 HTTPS 目标，支持签名事件。'],
  ['beta', 'Workflow & Queue', '声明式步骤与 SQLite 异步执行。'],
  ['experimental', 'Plugin & Shell', '仅执行配置声明的绝对路径。'],
  ['experimental', 'Agent & MCP', '固定模型、提示词与 Tool allowlist。'],
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
            <p className="lead">MailRelay 将认证邮件解析为声明式 Command，并由受限 Handler 执行。任何邮件客户端都可成为远程控制端。</p>
            <Install />
            <div className="hero-actions"><Link href="/docs">阅读文档</Link><a href="#scenarios">查看黄金场景 ↓</a></div>
          </div>
          <Terminal />
        </section>

        <section className="manifesto landing-section">
          <div className="manifesto-inner">
            <p className="section-label">The big promise</p>
            <h2>The universal remote for everything you run.</h2>
            <p>One inbox. Every declared command.</p>
            <div className="manifesto-proof"><span>Authenticated.</span><span>Auditable.</span><span>Recoverable.</span></div>
          </div>
        </section>

        <section className="landing-section" id="scenarios"><div className="section-inner">
          <p className="section-label">Golden scenarios</p>
          <h2>Remote operations from any inbox.</h2>
          <p className="section-intro">面向低频、异步、需要审计的远程操作。</p>
          <div className="scenario-list">{scenarios.map((item, i) => <article className={`scenario ${i % 2 ? 'reverse' : ''}`} key={item.n}>
            <div className="scenario-copy"><span>{item.n}</span><h3>{item.title}</h3><p>{item.copy}</p></div>
            <EmailCard subject={item.subject} result={item.result} />
          </article>)}</div>
        </div></section>

        <section className="landing-section why-email"><div className="section-inner">
          <p className="section-label">Why email wins</p><h2>A universal transport, already deployed.</h2>
          <p className="section-intro">现有邮件基础设施提供身份、寻址、离线投递与跨设备访问。</p>
          <div className="advantage-grid">{emailAdvantages.map(([title, copy], i) => <article key={title}><span>0{i+1}</span><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div></section>

        <section className="landing-section"><div className="section-inner discovery-layout">
          <div><p className="section-label">Discovery is the interface</p><h2>Documentation generated from configuration.</h2><p className="section-intro">Command 名称、描述、参数与示例自动生成 Catalog、`help` 和 `help deploy`。</p><p className="proof-note">Catalog Hash 记录 Added、Removed 与 Updated。</p></div>
          <div className="mail-stack"><MailPreview subject="help" lines={['Available Commands', 'deploy   部署项目', 'backup   备份数据', 'summary  总结附件']} /><MailPreview subject="help deploy" lines={['deploy', 'Description  部署项目', 'Parameters   env* · version', 'Example      env=prod']} /></div>
        </div></section>

        <section className="landing-section"><div className="section-inner">
          <p className="section-label">The protocol</p><h2>Explicit boundaries from mail to execution.</h2>
          <p className="section-intro">Receiver、Parser、Router 与 Handler 各自保持单一职责。</p>
          <div className="protocol">{[['01','Mail','接收'],['02','Parser','规范化'],['03','Auth','认证去重'],['04','Router','解析命令'],['05','Handler','安全执行'],['06','Reply','审计回复']].map(([n,t,d]) => <div className="protocol-item" key={n}><span>{n}</span><strong>{t}</strong><small>{d}</small></div>)}</div>
        </div></section>

        <section className="landing-section comparison"><div className="section-inner">
          <p className="section-label">MailRelay vs. the usual remote controls</p><h2>Purpose-built for asynchronous control.</h2>
          <p className="section-intro">适用于声明式远程操作，不替代交互式终端或专业运维平台。</p>
          <div className="comparison-table"><div className="comparison-head"><span>方式</span><span>擅长</span><span>需要承担</span></div>{comparisons.map((row) => <div className={row[0] === 'MailRelay' ? 'featured' : ''} key={row[0]}>{row.map(cell => <span key={cell}>{cell}</span>)}</div>)}</div>
        </div></section>

        <section className="landing-section trust-section" id="security"><div className="section-inner">
          <p className="section-label">Trust is a runtime feature</p><p className="trust-kicker">Built to be safe by default.</p><h2>Security enforced on every execution.</h2>
          <p className="section-intro">运行时统一处理认证、幂等、网络边界、审计、重试与恢复。</p>
          <div className="trust-grid">{trust.map(([title, copy], i) => <article key={title}><span>0{i+1}</span><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div></section>

        <section className="landing-section" id="handlers"><div className="section-inner">
          <p className="section-label">One router, every horizon</p><h2>One contract, multiple execution models.</h2>
          <p className="section-intro">Handler 共享统一接口，并在 Discovery 中公开成熟度。</p>
          <div className="stories">{handlers.map(([tag,title,text]) => <article className="story" key={title}><span className={`maturity ${tag}`}>{tag}</span><h3>{title}</h3><p>{text}</p></article>)}</div>
        </div></section>

        <section className="landing-section final-cta"><div className="section-inner cta">
          <p className="section-label">Start with one command</p><h2>From inbox to automation in five minutes.</h2>
          <p className="section-intro">声明一个固定 HTTPS Command，发送邮件并验证回复。</p>
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
