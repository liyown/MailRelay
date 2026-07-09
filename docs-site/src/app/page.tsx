import Link from 'next/link';
import { absoluteUrl, jsonLd, site } from '@/lib/seo';

const scenarios = [
  { n: '01', title: 'ChatGPT 手机端调用外部 API', copy: '在 ChatGPT 里让它按模板发一封邮件，MailRelay 收到后调用你配置好的 HTTP 接口，再把结果回信。', subject: 'push', result: 'http · 200 OK' },
  { n: '02', title: '快捷指令触发家里或服务器上的命令', copy: 'iOS 快捷指令、NAS、低代码工具只要能发邮件，就能触发一条受限命令。服务端不用额外开放入口。', subject: 'archive-report', result: 'queue · accepted · #1842' },
  { n: '03', title: '把慢任务放进队列', copy: '备份、生成报告、同步数据这类任务可以进 Queue。失败会重试，耗尽后停在 dead letter，等你修好再 replay。', subject: 'backup photos', result: 'queued · retry 1/4' },
  { n: '04', title: '转发一段 HTTP 请求', copy: '邮件正文可以是一段 HTTP/1.1 请求报文。MailRelay 校验发件人和 Token 后，按 allowlist 转发。', subject: 'forward', result: 'http_request · 201 Created' },
];

const emailAdvantages = [
  ['到处都能发', '手机 App、ChatGPT、脚本、NAS 和企业终端基本都能发邮件。'],
  ['断网也能等', '邮件天然是异步投递。客户端不需要一直在线等接口返回。'],
  ['格式够用了', 'Subject 放命令名，Header 放 Token，正文放参数或 HTTP 请求。'],
  ['结果能回到人手里', '执行结果通过回信送回去，SQLite 里保留去重、审计和失败记录。'],
];

const comparisons = [
  ['SSH', '连到机器上操作', '暴露入口、管理密钥、准备客户端'],
  ['Chat bot', '平台内交互', '接 Bot、维护账号和在线通道'],
  ['Raw webhook', '程序直接调用', '自己补认证、重试、审计和说明文档'],
  ['Custom dashboard', '做一套专用界面', '持续开发、部署、登录和权限'],
  ['MailRelay', '用邮件触发受控动作', '命令目录、认证、去重、审计、回信都在运行时里'],
];

const trust = [
  ['发件人和 Token 都要对', 'allowlist 校验发件人，Token 用常量时间比较。'],
  ['同一封邮件只执行一次', 'Message-ID 会进入 SQLite claim，重复投递不会重复跑命令。'],
  ['回信失败不重跑命令', '执行结果写入 outbox。SMTP 临时失败只重试投递。'],
  ['HTTP 出站有边界', '目标主机来自配置，path/query 可以映射参数，DNS、IP 和重定向都会检查。'],
  ['失败会留下来', '队列任务和回复投递耗尽重试后停在 dead letter，需要人工 replay。'],
  ['敏感值不进审计', '密码、Token、完整正文和 sensitive 参数不会写进 SQLite 审计记录。'],
];

const handlers = [
  ['stable', 'HTTP / HTTP Request / Webhook', '调用配置好的接口，或者转发邮件里的 HTTP 请求。'],
  ['stable', 'Workflow & Queue', '把多条命令串起来，或者把任务放到 SQLite 队列里慢慢跑。'],
  ['experimental', 'Plugin & Shell', '运行固定路径的本地程序，输入输出受限，不经过 shell 拼接。'],
  ['experimental', 'Agent & MCP', '端点、模型、提示词和 Tool allowlist 都写在配置里。'],
];

export default function HomePage() {
  const structuredData = {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'WebSite',
        name: site.name,
        url: absoluteUrl('/'),
        description: site.description,
        inLanguage: 'zh-CN',
      },
      {
        '@type': 'SoftwareApplication',
        name: site.name,
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'macOS, Linux',
        url: absoluteUrl('/'),
        codeRepository: site.github,
        description: site.description,
        offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
      },
    ],
  };
  return (
    <div className="landing">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLd(structuredData) }} />
      <Header />
      <main>
        <section className="hero">
          <div>
            <span className="eyebrow"><b>NEW</b> HTTP request forwarding, workflow, queue</span>
            <h1>让能发邮件的客户端调用受控 API。</h1>
            <p className="lead">MailRelay 跑在你的机器上，收一封带 Token 的邮件，按配置调用 HTTP、Webhook、Workflow 或 Queue，然后把执行结果回信。很适合把 ChatGPT 手机端、快捷指令、NAS 脚本接到你自己的 API。</p>
            <Install />
            <div className="hero-actions"><Link href="/docs">阅读文档</Link><a href="#scenarios">看几个用法 ↓</a></div>
          </div>
          <Terminal />
        </section>

        <section className="manifesto landing-section">
          <div className="manifesto-inner">
            <p className="section-label">What it does</p>
            <h2>收邮件、验身份、跑命令、写记录、回信。</h2>
            <p>认证、去重、审计、重试、outbox 和 dead letter 都在一条执行链路里。</p>
            <div className="manifesto-proof"><span>Sender + token</span><span>SQLite state</span><span>Operator replay</span></div>
          </div>
        </section>

        <section className="landing-section" id="scenarios"><div className="section-inner">
          <p className="section-label">Use cases</p>
          <h2>从手机、ChatGPT 或脚本发一封邮件。</h2>
          <p className="section-intro">MailRelay 适合低频、异步、需要留痕的动作。每个客户端都可以沿用发邮件这条路。</p>
          <div className="scenario-list">{scenarios.map((item, i) => <article className={`scenario ${i % 2 ? 'reverse' : ''}`} key={item.n}>
            <div className="scenario-copy"><span>{item.n}</span><h3>{item.title}</h3><p>{item.copy}</p></div>
            <EmailCard subject={item.subject} result={item.result} />
          </article>)}</div>
        </div></section>

        <section className="landing-section why-email"><div className="section-inner">
          <p className="section-label">Email as input</p><h2>邮件是一个很普通、但很好接的入口。</h2>
          <p className="section-intro">很多环境不能直接调你的内网 API，但可以发邮件。MailRelay 把这封邮件变成受限命令。</p>
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
          <p className="section-label">Where it fits</p><h2>它处理“发起一个动作”这件事。</h2>
          <p className="section-intro">适合声明式命令、固定目标和审计回放。需要交互式排障时，SSH 或专业平台还是更合适。</p>
          <div className="comparison-table"><div className="comparison-head"><span>方式</span><span>擅长</span><span>需要承担</span></div>{comparisons.map((row) => <div className={row[0] === 'MailRelay' ? 'featured' : ''} key={row[0]}>{row.map(cell => <span key={cell}>{cell}</span>)}</div>)}</div>
        </div></section>

        <section className="landing-section trust-section" id="security"><div className="section-inner">
          <p className="section-label">Runtime checks</p><p className="trust-kicker">每封邮件都要过同一套检查。</p><h2>能跑什么、发到哪里，都写在配置里。</h2>
          <p className="section-intro">发件人、Token、Message-ID、出站网络、审计、重试和回放由运行时统一执行。</p>
          <div className="trust-grid">{trust.map(([title, copy], i) => <article key={title}><span>0{i+1}</span><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div></section>

        <section className="landing-section" id="handlers"><div className="section-inner">
          <p className="section-label">Handlers</p><h2>收到命令后，可以交给不同的 Handler。</h2>
          <p className="section-intro">常用的 HTTP、HTTP Request、Webhook、Queue 和 Workflow 已经稳定；本地进程、Agent 与 MCP 仍标为实验能力。</p>
          <div className="stories">{handlers.map(([tag,title,text]) => <article className="story" key={title}><span className={`maturity ${tag}`}>{tag}</span><h3>{title}</h3><p>{text}</p></article>)}</div>
        </div></section>

        <section className="landing-section final-cta"><div className="section-inner cta">
          <p className="section-label">Try it</p><h2>配置一个 HTTP 命令，给它发封邮件。</h2>
          <p className="section-intro">初始化配置，检查策略，启动服务，然后从手机、ChatGPT 或脚本发出第一封命令邮件。</p>
          <div className="quickstart"><div><span>01</span><code>mailrelay init</code></div><div><span>02</span><code>mailrelay doctor</code></div><div><span>03</span><code>mailrelay run</code></div></div>
          <div className="cta-actions"><Link className="primary-button" href="/docs/getting-started/installation">开始使用</Link><Link className="secondary-button" href="/docs/getting-started/first-command">创建第一个命令</Link></div>
        </div></section>
      </main>
      <footer className="landing-footer"><span>mailrelay. · becomeopc</span><span>Email → Command → Execution → Response</span></footer>
    </div>
  );
}

function Header() { return <header className="site-header"><nav className="landing-nav"><Link className="brand" href="/">mailrelay.</Link><div className="nav-links"><a href="#scenarios">用法</a><a href="#security">安全</a><a href="#handlers">Handlers</a><Link href="/docs">文档</Link></div><span className="nav-spacer"/><a className="github-link" href="https://github.com/liyown/MailRelay">GitHub</a></nav></header>; }
function Install() { return <div className="install"><div className="install-tabs"><span className="active">Go</span><span>macOS</span><span>Linux</span></div><div className="install-command"><span>$</span><code>go install github.com/becomeopc/opc-mailrelay/cmd/mailrelay@latest</code></div></div>; }
function Terminal() { return <div className="terminal"><div className="window-dots"><i/><i/><i/></div><pre><b>Subject</b>   push{`\n`}<b>From</b>      me@example.com{`\n`}<b>Client</b>    ChatGPT iOS{`\n`}<b>Token</b>     verified{`\n\n`}<span># parser</span>     message=hello{`\n`}<span># router</span>     push → http{`\n`}<span># request</span>    GET api.example.com/push/hello{`\n`}<span># audit</span>      saved to SQLite{`\n`}<span># response</span>   SMTP reply queued</pre></div>; }
function EmailCard({ subject, result }: { subject: string; result: string }) { return <div className="email-card"><div className="email-meta"><span>From</span><b>me@example.com</b><span>Subject</span><b>{subject}</b></div><div className="email-body"><span>message authenticated</span><strong>{result}</strong><small>Reply queued · audit saved</small></div></div>; }
function MailPreview({ subject, lines }: { subject: string; lines: string[] }) { return <div className="mail-preview"><div><span>Subject</span><strong>{subject}</strong></div><pre>{lines.join('\n')}</pre></div>; }
