import { expect, loginAsAdmin, openTab, test } from './helpers';

for (const viewport of [{ width: 390, height: 740 }, { width: 1024, height: 600 }]) {
  test(`Kubernetes node images stay visible without zooming at ${viewport.width}px`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await page.route('**/api/node-images', route => route.fulfill({
      json: { success: true, data: [{ id: 1, name: 'Talos test image', provider: 'talos', version: 'v1.10.0', downloaded: true }] },
    }));
    await page.addInitScript(() => localStorage.setItem('bootimus_theme', 'dark'));
    await loginAsAdmin(page);
    await openTab(page, 'clusters');

    await expect(page.getByRole('cell', { name: 'Talos test image' })).toBeInViewport();
    const tab = page.locator('#clusters-tab');
    await expect(tab).toHaveCSS('overflow-y', 'auto');
    await expect(tab).toHaveCSS('color-scheme', 'dark');
    await expect(tab).not.toHaveCSS('scrollbar-color', 'auto');
    const nodes = tab.getByRole('heading', { name: 'Cluster Nodes' });
    await nodes.scrollIntoViewIfNeeded();
    await expect(nodes).toBeInViewport();
  });
}

test('an ISO properties request can fail without logging out a valid session', async ({ page }) => {
  const filename = 'session-test.iso';
  const name = 'Session test image';
  await page.route(/\/api\/images$/, route => route.fulfill({
    json: { success: true, data: [{ name, filename, distro: 'debian', boot_method: 'kernel', extracted: true }] },
  }));
  await page.route('**/api/images/boot-candidates?*', route => route.fulfill({
    status: 401, json: { success: false, error: 'Image request unauthorized' },
  }));
  await loginAsAdmin(page);
  await openTab(page, 'images');
  const rejectedRequest = page.waitForResponse(response => response.url().includes('/api/images/boot-candidates?'));
  await page.locator('#images-table').getByRole('cell', { name, exact: true }).click();

  expect((await rejectedRequest).status()).toBe(401);
  await expect(page.locator('#image-properties-modal')).toBeVisible();
  await expect(page.locator('#login-screen')).toBeHidden();
  expect(await page.evaluate(() => localStorage.getItem('bootimus_token'))).toBeTruthy();
});

test('a rejected session still returns to the login screen', async ({ page }) => {
  await loginAsAdmin(page);
  for (const path of ['me', 'scan']) {
    await page.route(`**/api/${path}`, route => route.fulfill({
      status: 401, json: { success: false, error: 'Invalid or expired token' },
    }));
  }
  await page.locator('.sidebar-nav [data-tab="images"]').click();
  await page.locator('#images-tab').getByRole('button', { name: 'Scan for ISOs', exact: true }).click();
  await expect(page.locator('#login-screen')).toBeVisible();
  expect(await page.evaluate(() => localStorage.getItem('bootimus_token'))).toBeNull();
});

test('Get Images offers official Debian, Kali and Proxmox downloads', async ({ page }) => {
  await loginAsAdmin(page);
  await openTab(page, 'images');
  await page.getByRole('button', { name: 'Get Images', exact: true }).click();
  const catalogue = page.locator('#get-images-list');
  await expect(catalogue.locator('#get-iso-url-debian-1')).toHaveValue(
    'https://cdimage.debian.org/debian-cd/13.8.0/amd64/iso-cd/debian-13.8.0-amd64-netinst.iso',
  );
  await expect(catalogue.locator('#get-iso-url-kali-0')).toHaveValue(
    'https://cdimage.kali.org/kali-2026.3/kali-linux-2026.3-installer-amd64.iso',
  );
  await expect(catalogue.locator('#get-iso-url-proxmox-0')).toHaveValue(
    'https://enterprise.proxmox.com/iso/proxmox-ve_9.2-1.iso',
  );
  await page.locator('#get-iso-filter').fill('Proxmox Backup Server');
  await expect(catalogue.locator('#get-iso-url-proxmox-2')).toHaveValue(
    'https://enterprise.proxmox.com/iso/proxmox-backup-server_4.2-1.iso',
  );
  await expect(catalogue.locator('#get-iso-url-proxmox-3')).toHaveValue(
    'https://enterprise.proxmox.com/iso/proxmox-backup-server_3.4-1.iso',
  );
});
