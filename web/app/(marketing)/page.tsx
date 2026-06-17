import Link from 'next/link';
import styles from './marketing.module.css';

export const runtime = 'edge';

const METRICS = [
  { label: 'P95 API Latency', value: '< 200ms' },
  { label: 'Edge-ready Services', value: '20+' },
  { label: 'End-to-end Coverage', value: '59.7%' },
  { label: 'Scale Architecture', value: 'K8s + VMs' },
];

const FEATURES = [
  {
    title: 'Compute Across Backends',
    body: 'Run container workloads with Docker or full virtual machines with Libvirt/KVM under one API contract.',
  },
  {
    title: 'Distributed S3-Compatible Storage',
    body: 'Use consistent hashing, gossip discovery, replication quorum, and object versioning for resilient storage.',
  },
  {
    title: 'Software-Defined Networking',
    body: 'Build isolated VPCs with OVS, subnet control, peering, and elastic IP management.',
  },
  {
    title: 'Managed Platform Services',
    body: 'Provision databases, caches, queues, serverless functions, and cloud-native workflows from one control plane.',
  },
  {
    title: 'Operations and Security',
    body: 'API key auth, tenant boundaries, RBAC and IAM, audit trails, and observability by default.',
  },
  {
    title: 'Production Delivery Stack',
    body: 'Deploy through Docker and Kubernetes with built-in health checks, metrics, tracing, and worker orchestration.',
  },
];

const JOURNEY = [
  {
    tag: 'Provision',
    title: 'Declare Infrastructure Once',
    body: 'Launch instances, volumes, networks, and managed services through a single API and tenant-aware model.',
  },
  {
    tag: 'Operate',
    title: 'Observe and Automate Continuously',
    body: 'Track health and events in real time while workers handle failover, scaling, and long-running orchestration.',
  },
  {
    tag: 'Evolve',
    title: 'Expand to Multi-Region Scale',
    body: 'Route traffic globally, add clusters, and grow service surface without redesigning the architecture.',
  },
];

const SIGNALS = [
  { name: 'Control Plane', value: 'Healthy' },
  { name: 'Storage Nodes', value: 'Replica Quorum' },
  { name: 'Event Stream', value: 'Realtime' },
  { name: 'Worker Fleet', value: 'Autoscaled' },
];

function BlueStripe() {
  return (
    <section className={styles.blueBannerWrap} aria-hidden="true">
      <div className={styles.blueBanner}>
        <div className={`${styles.blueTexture} ${styles.blueTextureLeft}`} />
        <div className={styles.blueGlyph}>✶</div>
        <div className={`${styles.blueTexture} ${styles.blueTextureRight}`} />
      </div>
    </section>
  );
}

export default function HomePage() {
  return (
    <main className={styles.page}>
      <section className={styles.hero}>
        <div className={styles.heroLeft}>
          <span className={styles.kicker}>Open Source Cloud Platform</span>
          <h1 className={styles.title}>Run Your Cloud, Not Someone Else&apos;s Rules.</h1>
          <p className={styles.subtitle}>
            The Cloud is a full-stack infrastructure platform with compute, storage, networking, and managed
            services. Self-host it, adapt it, and ship production-grade cloud features with total control.
          </p>
          <div className={styles.heroActions}>
            <Link href="/dashboard" className={`${styles.actionLink} ${styles.actionLinkPrimary}`}>
              Open Console
            </Link>
            <Link href="/pricing" className={styles.actionLink}>
              Explore Plans
            </Link>
          </div>
        </div>

        <aside className={styles.heroPanel}>
          {SIGNALS.map((signal) => (
            <div key={signal.name} className={styles.signalCard}>
              <span className={styles.signalTitle}>{signal.name}</span>
              <strong className={styles.signalValue}>{signal.value}</strong>
            </div>
          ))}
        </aside>
      </section>

      <section className={styles.metricsStrip}>
        {METRICS.map((metric) => (
          <article key={metric.label} className={styles.metric}>
            <div className={styles.metricLabel}>{metric.label}</div>
            <div className={styles.metricValue}>{metric.value}</div>
          </article>
        ))}
      </section>

      <BlueStripe />

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Everything Needed For a Real Cloud Surface</h2>
        <p className={styles.sectionLead}>
          Purpose-built around clean architecture and modular adapters, so you can extend capabilities without
          rewriting core services.
        </p>
        <div className={styles.featureGrid}>
          {FEATURES.map((feature) => (
            <article key={feature.title} className={styles.featureCard}>
              <h3>{feature.title}</h3>
              <p>{feature.body}</p>
            </article>
          ))}
        </div>
      </section>

      <BlueStripe />

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>From Provisioning To Global Runtime</h2>
        <p className={styles.sectionLead}>A practical workflow to launch and scale cloud services with confidence.</p>
        <div className={styles.storyGrid}>
          {JOURNEY.map((step) => (
            <article key={step.title} className={styles.storyStep}>
              <span className={styles.stepTag}>{step.tag}</span>
              <h3>{step.title}</h3>
              <p>{step.body}</p>
            </article>
          ))}
        </div>
      </section>

      <BlueStripe />

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Global Footprint By Design</h2>
        <p className={styles.sectionLead}>
          Build regional and global routing with distributed storage and traffic policies aligned to performance.
        </p>
        <div className={styles.regionMap}>
          <div className={styles.mapGrid} />
          <span className={styles.regionDot} style={{ top: '25%', left: '18%' }} />
          <span className={styles.regionDot} style={{ top: '35%', left: '34%' }} />
          <span className={styles.regionDot} style={{ top: '30%', left: '50%' }} />
          <span className={styles.regionDot} style={{ top: '46%', left: '72%' }} />
          <span className={styles.regionDot} style={{ top: '66%', left: '84%' }} />
        </div>
      </section>

      <section className={styles.cta}>
        <div className={styles.ctaCard}>
          <div>
            <h2 className={styles.ctaTitle}>Start Building On Infrastructure You Actually Own</h2>
            <p className={styles.ctaLead}>
              Open source, service-rich, and architecture-first. Deploy your own cloud control plane in minutes.
            </p>
          </div>
          <div className={styles.heroActions}>
            <Link href="/dashboard" className={`${styles.actionLink} ${styles.actionLinkDark}`}>
              Launch Console
            </Link>
            <Link href="/pricing" className={`${styles.actionLink} ${styles.actionLinkDark}`}>
              Review Pricing
            </Link>
          </div>
        </div>
      </section>

      <footer className={styles.footer}>Copyright 2026 The Cloud. Built for ownership, portability, and scale.</footer>
    </main>
  );
}
