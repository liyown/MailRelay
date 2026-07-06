import Link from 'next/link';

const protocol = [
  ['01', 'Mail', '接收原始邮件'], ['02', 'Parser', '生成请求'], ['03', 'Auth', '认证与去重'],
  ['04', 'Router', '解析命令'], ['05', 'Handler', '安全执行'], ['06', 'Reply', '审计并回复'],
];

const handlers = [
  ['stable', 'HTTP & Webhook', '固定 HTTPS 目标、域名白名单、DNS/IP 检查和重定向保护，避免成为 SSRF 工具。'],
  ['beta', 'Workflow & Queue', '组合已声明命令，使用 SQLite lease、有限重试和显式死信重放保持可恢复。'],
  ['experimental', 'Plugin & Shell', '只运行配置声明的绝对路径，不经过 shell 解释器，参数逐项扩展。'],
  ['experimental', 'Agent & MCP', 'Endpoint、模型、系统提示和 Tool allowlist 全部由配置固定，邮件无法越权覆盖。'],
];

export default function HomePage() {
  return (
    <div className="landing">
      <header className="site-header"><nav className="landing-nav">
        <Link className="brand" href="/">mailrelay.</Link>
        <div className="nav-links"><Link href="/docs">文档</Link><a href="#handlers">Handlers</a><a href="#security">安全</a></div>
        <span className="nav-spacer" />
        <a className="github-link" href="https://github.com/liyown/MailRelay">GitHub</a>
      </nav></header>

      <main>
        <section className="hero">
          <div>
            <span className="eyebrow"><b>NEW</b> single-host automation</span>
            <h1>Email is the command line for <em>everything</em> you run.</h1>
            <p className="lead">MailRelay 把每一封经过认证的邮件转换为声明式 Command，再交给安全、可组合的 Handler 执行。任何能发邮件的设备，都可以成为你的远程控制终端。</p>
            <div className="install"><div className="install-tabs"><span className="active">Go</span><span>macOS</span><span>Linux</span></div><div className="install-command"><span>$</span><code>go install github.com/becomeopc/opc-mailrelay/cmd/mailrelay@latest</code></div></div>
            <div className="hero-actions"><Link href="/docs">阅读文档</Link><a href="https://github.com/liyown/MailRelay">查看源码</a></div>
          </div>
          <Terminal />
        </section>

        <section className="landing-section"><div className="section-inner">
          <p className="section-label">The protocol</p><h2>From inbox to execution, every boundary stays small.</h2>
          <p className="section-intro">Receiver 不知道 HTTP，Router 不知道邮件，Handler 不知道 IMAP。每层只做一件事，因此新增命令只需要改配置。</p>
          <div className="protocol">{protocol.map(([n,t,d]) => <div className="protocol-item" key={n}><span>{n}</span><strong>{t}</strong><small>{d}</small></div>)}</div>
        </div></section>

        <section className="landing-section" id="handlers"><div className="section-inner">
          <p className="section-label">One interface</p><h2>One Router. Every Handler you need.</h2>
          <p className="section-intro">从稳定的 HTTP/Webhook 到可组合 Workflow、SQLite Queue，再到受限 Shell、Agent 和 MCP，全部通过统一接口接入。</p>
          <div className="stories">{handlers.map(([tag,title,text]) => <article className="story" key={title}><span>{tag}</span><h3>{title}</h3><p>{text}</p></article>)}</div>
        </div></section>

        <section className="landing-section" id="security"><div className="section-inner security-layout">
          <div><p className="section-label">Secure by design</p><h2>Built to be safe by default.</h2><p className="section-intro">危险能力默认关闭。认证、去重、超时、审计和回复重试不是插件，而是运行时的基本行为。</p></div>
          <ol className="checks"><li><span>01</span><div><strong>双重认证</strong><p>发件人 allowlist 与常量时间 Token 校验同时通过才会路由。</p></div></li><li><span>02</span><div><strong>零重复执行</strong><p>Message-ID 与 SQLite 原子 claim 在进程重启后仍然有效。</p></div></li><li><span>03</span><div><strong>回复与执行解耦</strong><p>SMTP Outbox 独立重试，不会再次调用 Handler。</p></div></li></ol>
        </div></section>

        <section className="landing-section"><div className="section-inner cta"><p className="section-label">Five minutes</p><h2>Start with one config file.</h2><p className="section-intro">初始化、检查、运行。所有命令、参数和 Discovery 文档都来自同一份 YAML。</p><div className="mini-terminal"><code>$ mailrelay init<br/>$ mailrelay doctor<br/>$ mailrelay run</code></div><Link className="primary-button" href="/docs">开始使用</Link></div></section>
      </main>
      <footer className="landing-footer"><span>mailrelay. · becomeopc</span><span>Email → Command → Execution → Response</span></footer>
    </div>
  );
}

function Terminal() {
  return <div className="terminal"><div className="window-dots"><i/><i/><i/></div><pre><b>Subject</b>   deploy{`\n`}<b>From</b>      ops@example.com{`\n`}<b>Token</b>     verified{`\n\n`}<span># parser</span>     mail → CommandRequest{`\n`}<span># router</span>     deploy → workflow{`\n`}<span># handler</span>    3 steps completed{`\n`}<span># audit</span>      saved to SQLite{`\n`}<span># response</span>   SMTP reply queued</pre></div>;
}
