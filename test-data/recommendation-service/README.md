# Recommendation Service

## Overview

ML-powered recommendation engine. Generates personalized product recommendations for users based on order history, browsing behavior, and collaborative filtering algorithms.

## Responsibilities

- Collaborative filtering (user-user, item-item similarity)
- Personalized product recommendations
- Trending products identification
- Model training and evaluation
- A/B testing for recommendation algorithms

## Key Dependencies

- **user-service**: User profiles and preferences
- **order-service**: Order history and purchase patterns
- **search-service**: Product search for similar items
- **database-service**: Historical order and user data for model training
- **logging-service**: Click-through rates and recommendation performance

## Model Architecture

- Collaborative filtering with matrix factorization
- Content-based similarity using product attributes
- Hybrid approach combining both methods
- Weekly model retraining on 30-day rolling window

## Recommendation Types

- "Customers who bought X also bought Y" (item-item similarity)
- "Recommended for you" (user-item personalization)
- "Trending now" (popular in last 7 days)
- "You might like" (collaborative filtering)

## Integration

Services call recommendation-service via REST API:
```
GET /recommendations?user_id=USR-123&count=10&category=electronics
```

## Performance

- Recommendation latency: <500ms
- Model training: 2 hours (weekly batch job)
- Accuracy: 25% CTR on recommendations
