# Metrics

Measure numeric data over time to define service-level indicators (SLI), objectives (SLO) and agreements (SLA).

Three kinds of metrics:

- Counters: Track the number of times an event happened & how many requests we have handled.
- Histograms: Distribution of data. Measure percentiles (a point in the distribution such that a certain percentage falls at or below that value) of request duration and size.
- Gauges: Track the current value of something like host's disk usage percentage or the number of load balancers.

> More about percentiles.
>
> Imagine we have different buckets covering different range of times like 0-10ms, 10-20ms, 20-50ms, etc.
>
> We put different requests in different buckets like bucket A has 10 requests, bucket B has 40 requests, etc.
>
> A percentile will tell us _how fast most requests are_. Example: p50 means 50% of the requests are this fast.

What to measure?

- Latency
- Traffic
- Errors
- Saturation: Measure of service's capacity like disk storage
