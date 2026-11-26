# Original project prompt in English

Use the `work-on-item` skill to complete the project foundation. Create an enterprise-grade e-commerce solution with the following features.

- Product CRUD with CSV import.
- A local database. Use PostgreSQL. Evaluate whether another type makes sense, although PostgreSQL's transaction support seems appropriate.
- Follow the product structure supplied in the CSV.
- Support searching for products and purchasing them. Simulate purchases without integrating a payment provider.
- Build the front end for product CRUD, search, and purchasing.

The application must run in Docker containers. Document all the decisions in this prompt, how the solution handles the main problems, and the alternatives considered in Markdown. Include instructions for running the application.

Split the system into at least four microservices to isolate workloads and keep services available. Add a cache to help prevent duplicate purchases and retain frequently or recently searched products. The challenge requires no pricing rules for discounts or surcharges. Use the following services.

- Catalog, including inventory.
- Search.
- Orders.
- Payments.

Use messaging for inter-service communication wherever possible. Choose the least burdensome lightweight option between technologies such as Kafka and RabbitMQ.

For the front end, consider React with Next.js and evaluate whether an SPA or another approach is better. Prefer the clean visual style of shadcn/ui.

Choose Java, PHP, or Go according to which best supports the event-driven architecture. Document the prompt and all decisions in English. Use `Code Challenge E-Commerce.csv` as the reference when needed.

## Continuation

Resume development with the `work-on-item` configuration after the addition of `.ai-development.json`. Use Ponytail and its skills.

## Frontend and payment clarification

React, Next.js, and shadcn/ui are the required frontend stack. Explain why Vite was used and correct the stack. Keep the sample products with script-like and SQL-like names as literal data and verify that those values cause no execution or unexpected side effects.

Hide the order UUID while confirming stock and after confirming the order. Open the simulated payment in a separate tab that looks like an external provider. Persist the order with pending status when the user proceeds to payment and provide a place to view orders and their current status.
