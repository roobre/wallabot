const SIGNATURE = 'Tm93IHRoYXQgeW91J3ZlIGZvdW5kIHRoaXMsIGFyZSB5b3UgcmVhZHkgdG8gam9pbiB1cz8gam9ic0B3YWxsYXBvcC5jb20==';

export function getSignature(url, method, timestamp) {
    try {
        if (url.includes('registration')) {
            url = url.split('?')[0];
        }
    }
    catch (e) {}

    var separator = '|';
    var signature = [method, url, timestamp].join(separator) + separator;

    return window.CryptoJS.enc.Base64.stringify(window.CryptoJS.HmacSHA256(signature, SIGNATURE));
}

//a: "GET|/api/v3/general/search|1566081935924|"
// ​
// arguments: Arguments
// ​
// e: "/api/v3/general/search"
// ​
// n: 1566081935924
// ​
// t: "GET"