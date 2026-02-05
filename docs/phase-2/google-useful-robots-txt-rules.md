# Useful robots.txt rules

Retrieved from: https://developers.google.com/crawling/docs/robots-txt/useful-robots-txt-rules



Here are some common useful robots.txt rules:

## 1. Disallow crawling of the entire site      

Keep in mind that in some situations URLs from the site may still be indexed, even          if they haven't been crawled.        

**Note**: This does not match the [various AdsBot crawlers](https://developers.google.com/crawling/docs/crawlers-fetchers/overview-google-crawlers), which must be named explicitly.        

```
User-agent: *
Disallow: /
```

## 2. Disallow crawling of a directory and its contents      

Append a forward slash to the directory name to disallow crawling of a whole directory.        

**Caution**: Remember, don't use robots.txt to block access to private content; use proper authentication instead. URLs disallowed by the robots.txt file might still be indexed without being crawled, and the robots.txt file can be viewed by anyone, potentially disclosing the location of your private content.        

```
User-agent: *
Disallow: /calendar/
Disallow: /junk/
Disallow: /books/fiction/contemporary/
```

## 3. Allow access to a single crawler      

Only `googlebot-news` may crawl the whole site.

```
User-agent: Googlebot-news
Allow: /

User-agent: *
Disallow: /
```

## 4. Allow access to all but a single crawler      

`Unnecessarybot` may not crawl the site, all other bots may.

```
User-agent: Unnecessarybot
Disallow: /

User-agent: *
Allow: /
```

## 5. Disallow crawling of a single web page

For example, disallow the `useless_file.html` page located at          `https://example.com/useless_file.html`, and other_useless_file.html` in the `junk` directory.        

```
User-agent: *
Disallow: /useless_file.html
Disallow: /junk/other_useless_file.html
```

## 6. Disallow crawling of the whole site except a subdirectory

Crawlers may only access the `public` subdirectory.

```
User-agent: *
Disallow: /
Allow: /public/
```

## 7. Block a specific image from Google Images

For example, disallow the `dogs.jpg` image.

```
User-agent: Googlebot-Image
Disallow: /images/dogs.jpg
```

## 8. Block all images on your site from Google Images

Google can't index images and videos without crawling them.

```
User-agent: Googlebot-Image
Disallow: /
```

## 9. Disallow crawling of files of a specific file type

For example, disallow for crawling all `.gif` files.

```
User-agent: Googlebot
Disallow: /*.gif$
```

## 10. Disallow crawling of an entire site, but allow `Mediapartners-Google`

This implementation hides your pages from search results, but the `Mediapartners-Google` web crawler can still analyze them to decide what ads to show visitors on your site.        

```
User-agent: *
Disallow: /

User-agent: Mediapartners-Google
Allow: /
```

## 11. Use the `*` and `$` wildcards to match URLs that end with a specific string      

For example, disallow all `.xls` files.

```
User-agent: Googlebot
Disallow: /*.xls$
```