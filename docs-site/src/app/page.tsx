import Link from 'next/link';

const scenarios = [
  { n: '01', title: '移动运维', copy: '手机发 deploy、backup 或 status，服务端按配置执行并回信。', subject: 'deploy', result: 'workflow · 3 steps · success' },
  { n: '02', title: '连接受限设备', copy: '设备只要能发邮件，就能触发固定命令，无需开放入口。', subject: 'archive-report', result: 'queue · accepted · #1842' },
  { n: '03', title: '可靠异步任务', copy: 'Queue 持久化任务、租约恢复、有限重试与 dead letter。', subject: 'backup photos', result: 'queued · retry 1/4' },
  { n: '04', title: '调用 Agent 与 MCP', copy: '模型、提示词、端点和 Tool allowlist 都由配置锁定。', subject: 'summary', result: 'agent → smtp reply' },
];

const emailAdvantages = [
  ['Universal', '手机、脚本、NAS 与企业终端都能发起同一套命令。'],
  ['Store & forward', '邮件可离线排队，网络恢复后继续送达。'],
  ['Rich payload', 'Subject、正文、Header 与附件承载参数和上下文。'],
  ['Human audit', '邮件会话、SQLite 审计与执行回复可相互追踪。'],
];

const comparisons = [
  ['SSH', '实时控制', '网络入口、密钥与客户端'],
  ['Chat bot', '即时交互', '平台账号、Bot 接入与在线通道'],
  ['Raw webhook', '机器调用', '需自行实现认证、重试与目录'],
  ['Custom dashboard', '定制体验', '持续开发、部署与维护'],
  ['MailRelay', '受限异步操作', '认证、去重、审计、重试与回信内置'],
];

const trust = [
  ['Two keys, one door', '发件人 allowlist 与常量时间 Token 校验同时生效。'],
  ['Idempotent at the edge', 'Message-ID 去重与 SQLite 原子 claim 降低重复执行风险。'],
  ['Execution is not delivery', 'Outbox 将执行结果与 SMTP 投递解耦。'],
  ['The network has boundaries', 'HTTP 固定 HTTPS 主机，并校验 DNS、IP 与重定向。'],
  ['Failure stays visible', '重试耗尽后保留在 dead letter，必须显式 replay。'],
  ['Secrets leave no trail', '凭证、完整正文与敏感参数不写入审计。'],
];

const handlers = [
  ['stable', 'HTTP & Webhook', '只调用配置中的固定 HTTPS 目标，遵守出站主机策略。'],
  ['beta', 'Workflow & Queue', '声明式步骤、持久队列、租约恢复与重试。'],
  ['experimental', 'Plugin & Shell', '固定绝对路径，无 shell 拼接，输入输出受限。'],
  ['experimental', 'Agent & MCP', '固定端点、模型、提示词和 Tool allowlist。'],
];

export default function HomePage() {
  return (
    <div className="landing">
      <Header />
      <main>
        <section className="hero">
          <div>
            <span className="eyebrow"><b>NEW</b> stable HTTP/Webhook path</span>
            <h1>用邮件执行可审计的远程操作。</h1>
            <p className="lead">MailRelay 在单机上接收认证邮件，将其解析为受限 Command，再交给配置声明的 Handler 执行。去重、审计、重试、dead letter 与回复投递都由 SQLite 持久化。</p>
            <Install />
            <div className="hero-actions"><Link href="/docs">阅读文档</Link><a href="#scenarios">查看黄金场景 ↓</a></div>
          </div>
          <Terminal />
        </section>

        <section className="manifesto landing-section">
          <div className="manifesto-inner">
            <p className="section-label">Operational baseline</p>
            <h2>稳定性不是插件，是运行时边界。</h2>
            <p>认证、去重、审计、重试、outbox 与 dead letter 在执行链路内置。</p>
            <div className="manifesto-proof"><span>Sender + token</span><span>SQLite state</span><span>Operator replay</span></div>
          </div>
        </section>

        <section className="landing-section" id="scenarios"><div className="section-inner">
          <p className="section-label">Golden scenarios</p>
          <h2>四个适合邮件触发的运维场景。</h2>
          <p className="section-intro">低频、异步、需要留痕的操作，比实时终端更适合邮件协议。</p>
          <div className="scenario-list">{scenarios.map((item, i) => <article className={`scenario ${i % 2 ? 'reverse' : ''}`} key={item.n}>
            <div className="scenario-copy"><span>{item.n}</span><h3>{item.title}</h3><p>{item.copy}</p></div>
            <EmailCard subject={item.subject} result={item.result} />
          </article>)}</div>
        </div></section>

        <section className="landing-section why-email"><div className="section-inner">
          <p className="section-label">Why email works</p><h2>邮件基础设施已经部署在每个设备上。</h2>
          <p className="section-intro">不用新客户端、不暴露 SSH 或 Dashboard；让已有邮箱承担认证入口和异步投递。</p>
          <div className="advantage-grid">{emailAdvantages.map(([title, copy], i) => <article key={title}><span>0{i+1}</span><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div></section>

        <section className="landing-section"><div className="section-inner discovery-layout">
          <div><p className="section-label">Command catalog</p><h2>命令说明来自同一份配置。</h2><p className="section-intro">Command 名称、参数、必填项与示例生成 Catalog、`help` 和 `help deploy`。</p><p className="proof-note">Catalog Hash 记录 Added、Removed 与 Updated。</p></div>
          <div className="mail-stack"><MailPreview subject="help" lines={['Available Commands', 'deploy   部署项目', 'backup   备份数据', 'summary  总结附件']} /><MailPreview subject="help deploy" lines={['deploy', 'Description  部署项目', 'Parameters   env* · version', 'Example      env=prod']} /></div>
        </div></section>

        <section className="landing-section"><div className="section-inner">
          <p className="section-label">The protocol</p><h2>从收信到执行，每一步都有边界。</h2>
          <p className="section-intro">Receiver、Parser、Auth、Router、Handler 与 Reply 分层处理，失败可记录、可恢复。</p>
          <div className="protocol">{[['01','Mail','接收'],['02','Parser','规范化'],['03','Auth','认证去重'],['04','Router','解析命令'],['05','Handler','安全执行'],['06','Reply','审计回复']].map(([n,t,d]) => <div className="protocol-item" key={n}><span>{n}</span><strong>{t}</strong><small>{d}</small></div>)}</div>
        </div></section>

        <section className="landing-section comparison"><div className="section-inner">
          <p className="section-label">MailRelay vs. the usual remote controls</p><h2>为异步控制设计，不替代终端。</h2>
          <p className="section-intro">适合声明式操作、固定目标和审计回放；交互式排障仍应使用 SSH 或专业平台。</p>
          <div className="comparison-table"><div className="comparison-head"><span>方式</span><span>擅长</span><span>需要承担</span></div>{comparisons.map((row) => <div className={row[0] === 'MailRelay' ? 'featured' : ''} key={row[0]}>{row.map(cell => <span key={cell}>{cell}</span>)}</div>)}</div>
        </div></section>

        <section className="landing-section trust-section" id="security"><div className="section-inner">
          <p className="section-label">Runtime safety</p><p className="trust-kicker">安全和恢复都在主流程里。</p><h2>每次执行都经过同一组约束。</h2>
          <p className="section-intro">发件人、Token、Message-ID、出站网络、审计、重试和回放由运行时统一执行。</p>
          <div className="trust-grid">{trust.map(([title, copy], i) => <article key={title}><span>0{i+1}</span><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div></section>

        <section className="landing-section" id="handlers"><div className="section-inner">
          <p className="section-label">Handler maturity</p><h2>一个 Router，多个受控执行模型。</h2>
          <p className="section-intro">HTTP/Webhook 是稳定路径；Queue、Workflow 和实验能力清楚标注成熟度。</p>
          <div className="stories">{handlers.map(([tag,title,text]) => <article className="story" key={title}><span className={`maturity ${tag}`}>{tag}</span><h3>{title}</h3><p>{text}</p></article>)}</div>
        </div></section>

        <section className="landing-section final-cta"><div className="section-inner cta">
          <p className="section-label">First stable command</p><h2>先跑通一个固定 HTTPS 命令。</h2>
          <p className="section-intro">初始化配置，检查策略，发送第一封命令邮件，再用状态和回复确认链路。</p>
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
