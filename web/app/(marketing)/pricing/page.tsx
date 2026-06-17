
import Link from 'next/link';
import styles from '../marketing.module.css';

export const runtime = 'edge';

const PLANS = [
  {
    name: 'Starter',
    price: '$0',
    meta: 'For local labs and early prototypes',
    features: ['Single user workspace', 'Core compute + storage APIs', 'Community support'],
  },
  {
    name: 'Pro',
    price: '$29',
    meta: 'Per workspace / month',
    features: [
      'Multi-tenant management',
      'Managed services surface',
      'Priority issue support',
      'Advanced monitoring views',
    ],
    featured: true,
  },
  {
    name: 'Enterprise',
    price: 'Custom',
    meta: 'Security + architecture partnership',
    features: ['Private onboarding', 'Dedicated support channel', 'Custom integrations', 'SLA-backed operations'],
  },
];

export default function PricingPage() {
  return (
    <main className={styles.page}>
      <section className={styles.section}>
        <h1 className={styles.sectionTitle}>Clear Plans For Every Stage</h1>
        <p className={styles.sectionLead}>
          Start free, scale when your platform grows, and move to enterprise when governance and operations demand it.
        </p>

        <div className={styles.pricingGrid}>
          {PLANS.map((plan) => (
            <article
              key={plan.name}
              className={`${styles.priceCard} ${plan.featured ? styles.priceFeatured : ''}`}
            >
              <h2 className={styles.priceName}>{plan.name}</h2>
              <p className={styles.priceValue}>{plan.price}</p>
              <p className={styles.priceMeta}>{plan.meta}</p>
              <ul className={styles.priceList}>
                {plan.features.map((feature) => (
                  <li key={feature}>- {feature}</li>
                ))}
              </ul>
            </article>
          ))}
        </div>
      </section>

      <section className={styles.cta}>
        <div className={styles.ctaCard}>
          <div>
            <h2 className={styles.ctaTitle}>Need a Guided Rollout?</h2>
            <p className={styles.ctaLead}>
              We can help map your migration path from public cloud dependencies to an owned cloud runtime.
            </p>
          </div>
          <div className={styles.heroActions}>
            <Link href="/dashboard" className={`${styles.actionLink} ${styles.actionLinkDark}`}>
              Continue to Console
            </Link>
            <Link href="/" className={`${styles.actionLink} ${styles.actionLinkDark}`}>
              Back to Overview
            </Link>
          </div>
        </div>
      </section>

      <footer className={styles.footer}>Need procurement details? Contact the maintainer team from the repository.</footer>
    </main>
  );
}
