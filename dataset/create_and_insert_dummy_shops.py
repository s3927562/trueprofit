# Generates dummy shops, inserts into DynamoDB,
# and saves merchant list + base revenue into dummy_shops.txt

import boto3
import random
import string
from datetime import datetime, timezone

# CONFIG
AWS_REGION = "us-east-1"

SHOP_TO_USER_TABLE = "TrueProfitShopToUser-dev"
INTEGRATIONS_TABLE = "TrueProfitIntegrations-dev"

USERNAME = "040894a8-c0b1-70f0-1033-d544610edb28"

NUM_SHOPS = 10
SHOP_PREFIX = "trueprofit-dummy"
SHOP_DOMAIN_SUFFIX = ".myshopify.com"

OUTPUT_FILE = "dummy_shops.txt"

# Base revenue range per shop
BASE_REV_MIN = 500.0
BASE_REV_MAX = 100000.0


def rand_suffix(n=6):
    return "".join(
        random.choice(string.ascii_lowercase + string.digits) for _ in range(n)
    )


def make_shop_name(idx: int) -> str:
    return f"{SHOP_PREFIX}-{idx:02d}-{rand_suffix(4)}{SHOP_DOMAIN_SUFFIX}"


def main():
    ddb = boto3.client("dynamodb", region_name=AWS_REGION)
    now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    shops_with_revenue = []

    for i in range(NUM_SHOPS):
        shop = make_shop_name(i + 1)
        base_rev = round(random.uniform(BASE_REV_MIN, BASE_REV_MAX), 2)
        shops_with_revenue.append((shop, base_rev))

        # Insert into ShopToUser table
        ddb.put_item(
            TableName=SHOP_TO_USER_TABLE,
            Item={
                "PK": {"S": f"SHOP#{shop}"},
                "SK": {"S": f"USER#{USERNAME}"},
                "Shop": {"S": shop},
                "UserSub": {"S": USERNAME},
                "CreatedAt": {"S": now},
            },
        )

        # Insert into Integrations table
        ddb.put_item(
            TableName=INTEGRATIONS_TABLE,
            Item={
                "PK": {"S": f"USER#{USERNAME}"},
                "SK": {"S": f"SHOPIFY#{shop}"},
                "Shop": {"S": shop},
            },
        )

    # Save output file
    with open(OUTPUT_FILE, "w") as f:
        for shop, rev in shops_with_revenue:
            f.write(f"{shop},{rev}\n")

    print(f"Inserted {len(shops_with_revenue)} shops for {USERNAME}")
    print(f"Saved list to: {OUTPUT_FILE}")
    print("\nShops generated:")
    for s, r in shops_with_revenue:
        print(f" - {s}  (base_revenue={r})")


if __name__ == "__main__":
    main()
